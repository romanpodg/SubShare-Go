package main

import (
	"database/sql"
	"strings"
	"time"

	"github.com/romanpodg/SubShare-Go/internal/model"
)

const listUsersQuery = `
		WITH device_agg AS (
			SELECT user_id,
			       COUNT(1) AS connected_devices,
			       GROUP_CONCAT(hwid, '||') AS connected_hwids
			FROM user_devices
			GROUP BY user_id
		),
		key_agg AS (
			SELECT user_id,
			       GROUP_CONCAT(key_id) AS assigned_key_ids
			FROM user_keys
			GROUP BY user_id
		)
		SELECT
			u.id, u.name, u.email, u.token,
			COALESCE(NULLIF(TRIM(u.time_zone), ''), 'Europe/Moscow') AS time_zone,
			COALESCE(NULLIF(TRIM(u.language), ''), 'ru') AS language,
			u.activation_code, u.subscription_id,
			u.subscription_name, COALESCE(NULLIF(u.subscription_refresh_hours, 0), 12) AS subscription_refresh_hours,
			u.subscription_info_url, u.subscription_extra_url, u.subscription_extra_status,
			u.activation_used_at, u.status, u.starts_at, u.expires_at,
			u.blocked_reason,
			COALESCE(u.max_devices, 0) AS max_devices,
			COALESCE(d.connected_devices, 0) AS connected_devices,
			COALESCE(d.connected_hwids, '') AS connected_hwids,
			COALESCE(NULLIF(TRIM(u.key_assignment_mode), ''), 'all') AS key_assignment_mode,
			u.created_at,
			COALESCE(k.assigned_key_ids, '') AS assigned_key_ids
		FROM users u
		LEFT JOIN device_agg d ON d.user_id = u.id
		LEFT JOIN key_agg k ON k.user_id = u.id
		ORDER BY u.id DESC
	`

func (a *App) listUsers() ([]model.User, error) {
	rows, err := a.db.Query(listUsersQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := readUserRows(rows)
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return out, nil
	}

	devicesByUser, err := a.loadConnectedDevicesByUser()
	if err != nil {
		return nil, err
	}

	attachConnectedDevices(out, devicesByUser)
	return out, nil
}

func readUserRows(rows *sql.Rows) ([]model.User, error) {
	var users []model.User
	for rows.Next() {
		user, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func attachConnectedDevices(users []model.User, devicesByUser map[int64][]model.ConnectedDevice) {
	for index := range users {
		devices := devicesByUser[users[index].ID]
		if devices == nil {
			devices = make([]model.ConnectedDevice, 0)
		}
		users[index].ConnectedDevices = devices
		if users[index].ConnectedHWIDs == nil {
			users[index].ConnectedHWIDs = make([]string, 0)
		}
	}
}

// userListRow keeps nullable database values separate from the API projection.
type userListRow struct {
	user                                                               model.User
	status, timeZone, language, activationCode                         sql.NullString
	subscriptionID, subscriptionName                                   sql.NullString
	subscriptionRefreshHours                                           sql.NullInt64
	subscriptionInfoURL, subscriptionExtraURL, subscriptionExtraStatus sql.NullString
	activationUsedAt, startsAt, expiresAt                              sql.NullTime
	blockedReason                                                      sql.NullString
	maxDevices, connectedDevices                                       sql.NullInt64
	connectedHWIDs, assignedKeyIDs                                     sql.NullString
}

// scanUserRow reads one row of the listUsers query and derives the display
// fields (localized inputs, effective status, split HWIDs).
func scanUserRow(rows *sql.Rows) (model.User, error) {
	var row userListRow
	if err := rows.Scan(
		&row.user.ID,
		&row.user.Name,
		&row.user.Email,
		&row.user.Token,
		&row.timeZone,
		&row.language,
		&row.activationCode,
		&row.subscriptionID,
		&row.subscriptionName,
		&row.subscriptionRefreshHours,
		&row.subscriptionInfoURL,
		&row.subscriptionExtraURL,
		&row.subscriptionExtraStatus,
		&row.activationUsedAt,
		&row.status,
		&row.startsAt,
		&row.expiresAt,
		&row.blockedReason,
		&row.maxDevices,
		&row.connectedDevices,
		&row.connectedHWIDs,
		&row.user.KeyAssignmentMode,
		&row.user.CreatedAt,
		&row.assignedKeyIDs,
	); err != nil {
		return row.user, err
	}
	return row.toUser(), nil
}

func (row userListRow) toUser() model.User {
	u := row.user
	u.ActivationCode = strings.TrimSpace(row.activationCode.String)
	u.TimeZone = firstNonEmpty(row.timeZone.String, "Europe/Moscow")
	u.Language = firstNonEmpty(row.language.String, "ru")
	u.SubscriptionID = strings.TrimSpace(row.subscriptionID.String)
	u.SubscriptionName = strings.TrimSpace(row.subscriptionName.String)
	u.SubscriptionRefreshHours = positiveIntOrDefault(row.subscriptionRefreshHours, 12)
	u.SubscriptionInfoURL = strings.TrimSpace(row.subscriptionInfoURL.String)
	u.SubscriptionExtraURL = strings.TrimSpace(row.subscriptionExtraURL.String)
	u.SubscriptionExtraStatus = strings.TrimSpace(row.subscriptionExtraStatus.String)
	row.setDisplayDates(&u)
	u.Status = model.NormalizeStoredStatus(row.status.String)
	u.BlockedReason = strings.TrimSpace(row.blockedReason.String)
	u.MaxDevices = positiveIntOrDefault(row.maxDevices, 0)
	u.ConnectedDeviceCount = positiveIntOrDefault(row.connectedDevices, 0)
	u.EffectiveStatus = model.EffectiveUserStatus(
		u.Status,
		row.expiresAt.Time,
		row.expiresAt.Valid,
		u.ConnectedDeviceCount,
		u.MaxDevices,
		time.Now(),
	)
	rawHWIDs := strings.TrimSpace(row.connectedHWIDs.String)
	if rawHWIDs != "" {
		u.ConnectedHWIDs = strings.Split(rawHWIDs, "||")
	}
	u.AssignedKeyIDs = strings.TrimSpace(row.assignedKeyIDs.String)
	return u
}

func (row userListRow) setDisplayDates(user *model.User) {
	location, err := time.LoadLocation(user.TimeZone)
	if err != nil {
		location = time.UTC
	}
	user.ActivationUsedAt = formatDateTimeInputInLocation(row.activationUsedAt, location)
	user.StartsAtInput = formatDateTimeInputInLocation(row.startsAt, location)
	user.ExpiresAtInput = formatDateTimeInputInLocation(row.expiresAt, location)
}
