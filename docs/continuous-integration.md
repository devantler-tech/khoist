# CI coverage

Pull requests that modify only the 23 reviewed, existing browser test files can
use the browser profile. The classifier runs from the event's exact base commit
in a separate checkout and examines the exact tested Git tree. Both commits must
retain the complete regular-file inventory, browser build constraint, external
test package and reviewed first-party imports. Browser additions, deletions,
renames, production code, assets, fixtures, dependency files, workflow changes
and unknown paths use full acceptance. Manual and merge-group runs also use the
full profile.

The `adr` profile admits only additions of regular mode-100644 UTF-8 Markdown
files matching `docs/adr/NNNN-lowercase-title.md`. Each Git object is bounded to
1 MiB; NUL bytes, executable files and symlinks are refused. Modified, deleted or
renamed ADRs, mixed change families and all other documentation retain full
acceptance. In particular, `docs/examples` contains executable fixtures, and
`docs/postgresql-age-image.md` supplies commands to an acceptance test. The audited
runtime readers do not consume ADR prose. Existing documentation, link and
security checks remain.

All profiles run controller/API tests, build, chart checks, core lint,
release-contract checks, the real browser suite and browser-tagged lint. The full
profile additionally runs all seven source clusters, native Document and Graph
acceptance, PostgreSQL model acceptance and AGE image acceptance. Classifier
errors or missing output retain these full-suite jobs.

The required summary reads its checker from the same trusted base. It requires
an exact job inventory, successful selection and every selected job's success.
Only the explicitly unselected heavy jobs may report skipped in a verified
browser or ADR profile. Missing, failed, cancelled, unknown or extra results fail
the summary. When the trusted base has no classifier yet, only the complete
successful suite can pass.

The classifier tests check the entire browser inventory and first-party
dependency closure, including packages that embed assets. Workflow regressions
bind every logical job to the required summary and keep the seven source matrix
members. Updating the reviewed inventory or classifier requires full acceptance.

A classifier/workflow change itself uses full native acceptance. The new route
takes effect only after protected delivery, when the event base contains it.
Source tests do not establish savings; record selected checks, declared skips,
exact base/tested identities and terminal feedback time on an eligible real ADR
addition before claiming a performance improvement.
