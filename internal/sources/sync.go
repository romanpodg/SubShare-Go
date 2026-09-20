package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/romanpodg/SubShare-Go/internal/keymanagement"
	"github.com/romanpodg/SubShare-Go/internal/model"
	"github.com/romanpodg/SubShare-Go/internal/profiles"
	"github.com/romanpodg/SubShare-Go/internal/storage"
)

// SyncTarget is what Sync needs to know about the source being synchronised.
type SyncTarget struct {
	ID            int64
	Enabled       bool
	KeyCategory   string
	KeyInsertMode string
}

// Sync reconciles the parsed feed with the source's stored keys inside the
// given store transaction: repairs historical duplicates, matches by ref,
// fingerprint or unique label, inserts/updates/removes rows and records the
// sync on the source. It never touches SQL or encryption directly.
func Sync(sync *storage.SourceSync, source SyncTarget, parsed ParseResult, fingerprintKeys [][]byte) (SyncResult, error) {
	state := newSyncState(sync, source, parsed, fingerprintKeys)
	if len(parsed.Keys) == 0 {
		return state.result, fmt.Errorf("no_keys_to_import")
	}
	if err := state.prepare(); err != nil {
		return state.result, err
	}
	if err := state.reconcile(); err != nil {
		return state.result, err
	}
	return state.result, state.finish()
}

// syncState carries the maps and counters shared by the Sync phases.
type syncState struct {
	sync            *storage.SourceSync
	source          SyncTarget
	parsed          ParseResult
	fingerprintKeys [][]byte
	result          SyncResult
	itemIndexes     map[string]int

	statusValue      string
	targetCategory   string
	targetCategoryID any

	existingKeys          []storage.SourceKey
	removedDuplicateIDs   map[int64]struct{}
	existingByRef         map[string]storage.SourceKey
	existingByFingerprint map[string]storage.SourceKey
	existingByLabel       map[string]storage.SourceKey
	existingLabelCounts   map[string]int
	existingIDs           map[int64]struct{}
	incomingLabelCounts   map[string]int

	seenRefs      map[string]struct{}
	nextSortOrder int64
}

func newSyncState(sync *storage.SourceSync, source SyncTarget, parsed ParseResult, fingerprintKeys [][]byte) *syncState {
	state := &syncState{sync: sync, source: source, parsed: parsed, fingerprintKeys: fingerprintKeys}
	state.result = SyncResult{Items: append([]ImportItem(nil), parsed.Items...)}
	state.result.Skipped = parsed.Counts.Rejected + parsed.Counts.Unsupported + parsed.Counts.Duplicate
	state.itemIndexes = make(map[string]int, len(state.result.Items))
	for index := range state.result.Items {
		state.itemIndexes[state.result.Items[index].ItemRef] = index
	}
	state.statusValue = model.KeyStatusActive
	if !source.Enabled {
		state.statusValue = model.KeyStatusNonActive
	}
	state.targetCategory = keymanagement.NormalizeKeyCategory(source.KeyCategory)
	return state
}

// prepare loads the stored side of the reconciliation: the target category,
// the existing rows (with historical duplicates repaired) and the lookup
// indexes, plus the sort order the first incoming key will receive.
func (s *syncState) keyRef(id int64) storage.SourceKeyRef {
	return storage.SourceKeyRef{SourceID: s.source.ID, ID: id}
}

func (s *syncState) prepare() error {
	targetCategoryID, err := s.sync.EnsureCategory(s.targetCategory)
	if err != nil {
		return err
	}
	s.targetCategoryID = targetCategoryID
	s.existingKeys, err = s.sync.ListSourceKeys(s.source.ID)
	if err != nil {
		return err
	}
	if len(s.fingerprintKeys) == 0 {
		return fmt.Errorf("profile_fingerprint_key_unavailable")
	}
	if err := s.repairDuplicateGroups(); err != nil {
		return err
	}
	s.indexExistingKeys()
	s.countIncomingLabels()
	s.seenRefs = make(map[string]struct{}, len(s.parsed.Keys))
	insertTop := NormalizeKeyInsertMode(s.source.KeyInsertMode) == "top"
	s.nextSortOrder, err = s.sync.NextSortOrder(targetCategoryID, s.source.ID, insertTop, len(s.parsed.Keys))
	return err
}

// reconcile upserts every incoming key and removes the rows the feed dropped.
func (s *syncState) reconcile() error {
	for index, item := range s.parsed.Keys {
		if err := s.upsertParsedKey(index, item); err != nil {
			return err
		}
	}
	return s.removeUnmatched()
}

// finish recomputes the counters from the final item statuses and records the
// sync on the source.
func (s *syncState) finish() error {
	removed := s.result.Counts.Removed
	s.result.Counts = CountItems(s.result.Items)
	s.result.Counts.Removed = removed
	metadata := s.parsed.Metadata
	return s.sync.MarkSourceSynced(s.source.ID, s.result.Imported, storage.SourceSyncMetadata{
		Title: metadata.Title, RefreshHours: metadata.RefreshHours, SupportURL: metadata.SupportURL,
		WebPageURL: metadata.WebPageURL, Announce: metadata.Announce,
	})
}

func (s *syncState) setItemStatus(itemRef, status string) {
	if index, exists := s.itemIndexes[itemRef]; exists {
		s.result.Items[index].Status = status
	}
}

// syncLabel is the stored label for an incoming key: trimmed, defaulted by
// position and truncated to the column width.
func syncLabel(item ParsedKey, index int) string {
	label := strings.TrimSpace(item.Label)
	if label == "" {
		label = fmt.Sprintf("Импорт %03d", index+1)
	}
	if len(label) > 255 {
		label = label[:255]
	}
	return label
}

// repairDuplicateGroups merges historical duplicate rows. Historical
// synchronizers could create multiple source-owned rows for the same URI
// profile. Repair those groups before normal matching so assigning the
// canonical fingerprint cannot temporarily conflict with a duplicate. A row
// already holding the current fingerprint wins; otherwise the lowest stable ID
// wins because existingKeys is ordered by ID. Raw XRAY-JSON does not parse as
// a URI profile and deliberately retains exact-raw identity.
func (s *syncState) repairDuplicateGroups() error {
	s.removedDuplicateIDs = make(map[int64]struct{})
	for fingerprint, indexes := range s.semanticFingerprintGroups() {
		if len(indexes) < 2 {
			continue
		}
		if err := s.mergeDuplicateGroup(fingerprint, indexes); err != nil {
			return err
		}
	}
	return nil
}

// semanticFingerprintGroups indexes existing rows by the fingerprint of their
// decrypted URI profile; rows that do not parse as one are left out.
func (s *syncState) semanticFingerprintGroups() map[string][]int {
	groups := make(map[string][]int)
	for index := range s.existingKeys {
		if s.existingKeys[index].URL == "" {
			continue
		}
		profile, parseErr := profiles.Parse(s.existingKeys[index].URL)
		if parseErr != nil {
			continue
		}
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, s.fingerprintKeys[0])
		if fingerprintErr != nil {
			continue
		}
		groups[fingerprint] = append(groups[fingerprint], index)
	}
	return groups
}

func (s *syncState) mergeDuplicateGroup(fingerprint string, indexes []int) error {
	survivorIndex := s.duplicateSurvivorIndex(fingerprint, indexes)
	survivorID := s.existingKeys[survivorIndex].ID
	if err := s.inheritClientDisplayName(survivorIndex, indexes); err != nil {
		return err
	}
	for _, index := range indexes {
		if err := s.removeDuplicateRow(survivorID, s.existingKeys[index].ID); err != nil {
			return err
		}
	}
	if s.existingKeys[survivorIndex].Fingerprint == fingerprint {
		return nil
	}
	if err := s.sync.SetFingerprint(s.keyRef(survivorID), fingerprint); err != nil {
		return err
	}
	s.existingKeys[survivorIndex].Fingerprint = fingerprint
	return nil
}

// duplicateSurvivorIndex prefers the row already holding the fingerprint and
// otherwise the lowest stable ID.
func (s *syncState) duplicateSurvivorIndex(fingerprint string, indexes []int) int {
	for _, index := range indexes {
		if s.existingKeys[index].Fingerprint == fingerprint {
			return index
		}
	}
	return indexes[0]
}

// removeDuplicateRow folds duplicateID's user assignments into the survivor
// and deletes the duplicate; the survivor itself is left alone.
func (s *syncState) removeDuplicateRow(survivorID, duplicateID int64) error {
	if duplicateID == survivorID {
		return nil
	}
	if err := s.sync.MergeUserAssignments(survivorID, duplicateID); err != nil {
		return err
	}
	if err := s.sync.DeleteSourceKey(s.keyRef(duplicateID)); err != nil {
		return err
	}
	s.removedDuplicateIDs[duplicateID] = struct{}{}
	s.result.Counts.Removed++
	return nil
}

// inheritClientDisplayName: client_display_name is local administrator
// metadata, not source data. Keep the survivor's override when present;
// otherwise inherit the first non-empty override in stable-ID order before
// duplicates are deleted.
func (s *syncState) inheritClientDisplayName(survivorIndex int, indexes []int) error {
	if strings.TrimSpace(s.existingKeys[survivorIndex].ClientDisplayName) != "" {
		return nil
	}
	for _, index := range indexes {
		override := strings.TrimSpace(s.existingKeys[index].ClientDisplayName)
		if override == "" {
			continue
		}
		if err := s.sync.SetClientDisplayName(s.keyRef(s.existingKeys[survivorIndex].ID), override); err != nil {
			return err
		}
		s.existingKeys[survivorIndex].ClientDisplayName = override
		return nil
	}
	return nil
}

func (s *syncState) indexExistingKeys() {
	s.existingByRef = make(map[string]storage.SourceKey)
	s.existingByFingerprint = make(map[string]storage.SourceKey)
	s.existingByLabel = make(map[string]storage.SourceKey)
	s.existingLabelCounts = make(map[string]int)
	s.existingIDs = make(map[int64]struct{})
	for _, key := range s.existingKeys {
		if _, removed := s.removedDuplicateIDs[key.ID]; removed {
			continue
		}
		if key.Ref != "" {
			s.existingByRef[key.Ref] = key
		}
		if key.Fingerprint != "" {
			if _, exists := s.existingByFingerprint[key.Fingerprint]; !exists {
				s.existingByFingerprint[key.Fingerprint] = key
			}
		} else if key.URL != "" {
			s.indexLegacyFingerprints(key)
		}
		label := strings.TrimSpace(key.Label)
		if label != "" {
			s.existingLabelCounts[label]++
			s.existingByLabel[label] = key
		}
		s.existingIDs[key.ID] = struct{}{}
	}
}

// indexLegacyFingerprints: rows imported before semantic fingerprints existed
// still need to preserve their ID on the first post-upgrade
// rename/re-encoding. Compute only in memory from the decrypted profile; raw
// XRAY-JSON intentionally remains on its exact-raw reference identity.
func (s *syncState) indexLegacyFingerprints(key storage.SourceKey) {
	profile, parseErr := profiles.Parse(key.URL)
	if parseErr != nil {
		return
	}
	for _, fingerprintKey := range s.fingerprintKeys {
		fingerprint, fingerprintErr := profiles.Fingerprint(profile, fingerprintKey)
		if fingerprintErr != nil {
			continue
		}
		if _, exists := s.existingByFingerprint[fingerprint]; !exists {
			s.existingByFingerprint[fingerprint] = key
		}
	}
}

func (s *syncState) countIncomingLabels() {
	s.incomingLabelCounts = make(map[string]int)
	for index, item := range s.parsed.Keys {
		s.incomingLabelCounts[syncLabel(item, index)]++
	}
}

func (s *syncState) upsertParsedKey(index int, item ParsedKey) error {
	ref := strings.TrimSpace(item.Ref)
	if ref == "" {
		ref = KeyRef(item.URL)
	}
	if _, exists := s.seenRefs[ref]; exists {
		s.result.Skipped++
		s.setItemStatus(item.ItemRef, StatusDuplicate)
		return nil
	}
	s.seenRefs[ref] = struct{}{}

	label := syncLabel(item, index)
	urlValue := item.URL
	if strings.TrimSpace(urlValue) == "" || len(urlValue) > 65535 {
		s.result.Skipped++
		s.setItemStatus(item.ItemRef, StatusRejected)
		return nil
	}
	warningPayload, err := json.Marshal(item.WarningCodes)
	if err != nil {
		return err
	}
	write := storage.SourceKeyWrite{
		SourceID: s.source.ID, Ref: ref, Label: label, CategoryID: s.targetCategoryID, Category: s.targetCategory,
		SortOrder: s.nextSortOrder, Protocol: item.Protocol, Fingerprint: item.Fingerprint,
		ProfileSchemaVersion: item.ProfileSchemaVersion, Compatibility: item.Compatibility, WarningsJSON: string(warningPayload), URL: urlValue,
	}
	if existing, matched := s.matchExisting(item, ref, label); matched {
		return s.writeMatched(existing, item, write)
	}
	return s.insertNew(item, write)
}

// matchExisting finds the stored row for an incoming key by ref, then by any
// fingerprint candidate, then by unique label. A provider may rotate
// connection material while retaining the logical profile name. When both
// sides have exactly one such label, preserve the row (and its local
// metadata). Ambiguous labels deliberately remain on the conservative
// remove/add path.
func (s *syncState) matchExisting(item ParsedKey, ref, label string) (storage.SourceKey, bool) {
	if existing, matched := s.existingByRef[ref]; matched {
		return existing, true
	}
	for _, candidate := range item.FingerprintCandidates {
		if existing, exists := s.existingByFingerprint[candidate]; exists {
			return existing, true
		}
	}
	if s.incomingLabelCounts[label] == 1 && s.existingLabelCounts[label] == 1 {
		candidate := s.existingByLabel[label]
		if _, available := s.existingIDs[candidate.ID]; available {
			return candidate, true
		}
	}
	return storage.SourceKey{}, false
}

// sourceKeyFields are the stored columns a refresh may change; comparing them
// decides between the updated and unchanged statuses.
type sourceKeyFields struct {
	Label                string
	URL                  string
	Protocol             string
	Fingerprint          string
	ProfileSchemaVersion int
	Compatibility        string
	WarningsJSON         string
}

func storedKeyFields(existing storage.SourceKey) sourceKeyFields {
	return sourceKeyFields{
		Label: existing.Label, URL: existing.URL, Protocol: existing.Protocol, Fingerprint: existing.Fingerprint,
		ProfileSchemaVersion: existing.ProfileSchemaVersion, Compatibility: existing.Compatibility, WarningsJSON: existing.WarningsJSON,
	}
}

func incomingKeyFields(write storage.SourceKeyWrite) sourceKeyFields {
	return sourceKeyFields{
		Label: write.Label, URL: write.URL, Protocol: write.Protocol, Fingerprint: write.Fingerprint,
		ProfileSchemaVersion: write.ProfileSchemaVersion, Compatibility: write.Compatibility, WarningsJSON: write.WarningsJSON,
	}
}

func (s *syncState) writeMatched(existing storage.SourceKey, item ParsedKey, write storage.SourceKeyWrite) error {
	if existing.Ref != "" {
		write.Ref = existing.Ref
	}
	changed := storedKeyFields(existing) != incomingKeyFields(write)
	if err := s.sync.UpdateSourceKey(existing.ID, write); err != nil {
		return err
	}
	delete(s.existingIDs, existing.ID)
	s.nextSortOrder++
	s.result.Imported++
	if changed {
		s.setItemStatus(item.ItemRef, StatusUpdated)
	} else {
		s.setItemStatus(item.ItemRef, StatusUnchanged)
	}
	return nil
}

func (s *syncState) insertNew(item ParsedKey, write storage.SourceKeyWrite) error {
	write.Status = s.statusValue
	_, err := s.sync.InsertSourceKey(write)
	if errors.Is(err, storage.ErrDuplicateSourceKey) {
		s.result.Skipped++
		s.setItemStatus(item.ItemRef, StatusDuplicate)
		return nil
	}
	if err != nil {
		return err
	}
	s.setItemStatus(item.ItemRef, StatusAdded)
	s.nextSortOrder++
	s.result.Imported++
	return nil
}

// removeUnmatched deletes stored rows the feed no longer lists. Any
// rejected/unsupported source item makes absence ambiguous. Preserve unmatched
// rows until a fully parsed refresh confirms they are missing.
func (s *syncState) removeUnmatched() error {
	if s.parsed.Counts.Rejected != 0 || s.parsed.Counts.Unsupported != 0 {
		return nil
	}
	for keyID := range s.existingIDs {
		if err := s.sync.DeleteSourceKey(s.keyRef(keyID)); err != nil {
			return err
		}
		s.result.Counts.Removed++
	}
	return nil
}
