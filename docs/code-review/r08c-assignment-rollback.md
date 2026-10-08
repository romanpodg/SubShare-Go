# R08c — Required profile assignment rollback correction

R06 reproduced successful profile creation after a user-assignment SQL failure, leaving a committed parent and encrypted secret without required all-mode assignments. R08c is a separate corrective change after R08; it changes the failed-command outcome while retaining successful creation behavior, the existing schema and one transaction per command.

## Correction and evidence

Pre-correction commit `48f5c20` replaces the defect characterization with a full-unit rollback regression. The focused test fails against the original behavior with `create ignored assignment failure`. The fixture injects an abort after the first all-mode user assignment and snapshots profiles, secrets, categories, source rows and user assignments.

`commitLocalProfileCreation` now requires the assignment statement to succeed before committing the same profile/secret transaction. Failure propagates a wrapped SQL error; the existing deferred rollback discards the entire unit. The method returns no key or URI and performs no post-commit read on assignment failure. Normal assignment SQL, creation/commit error classification and successful reload are retained. Clone behavior is unchanged.

The corrected storage regression passes (1.402s), verifies the complete snapshot is retained, drops the trigger and proves clean retry with both all-mode assignments. A separate HTTP fixture passes (1.413s): the failed create returns the existing generic `500/create_failed` envelope with no secret/SQL details, no success audit and no partial rows; retry returns 201, one success audit and the required assignments.

## Validation and acceptance

Vet, lint (zero new issues), server build, migration compatibility (3.553s) and strict CodeScene delta against the R08 branch pass. The mutation adapter remains at **7.71** with unchanged existing findings; the modified storage regression and new HTTP test file score **10.00**. No rule, gate or failure is suppressed. Full uncached backend regression is running with saved JSON under `.cache/r08c`; its own hosted checks and automated review must pass before merge.

After R08c acceptance, continue to R09's frontend preservation corpus and the remaining original batches. A01's broader mutation boundaries must also be reconciled with their acceptance criteria before the overall backlog is declared complete.
