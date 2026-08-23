# Table schema conventions

Use schema-free `table_*` with a logical **collection** name and JSON-ish fields in each row. Always include `book_id`.

Suggested collections (names are conventions — be consistent within a book):

## `characters`

`id`, `book_id`, `name`, `status` (`candidate|canon`), `role`, `desire`, `wound`, `traits`, `knowledge_boundary`, `location`, `notes`, `updated_ch`

## `locations`

`id`, `book_id`, `name`, `status`, `summary`, `rules_refs`

## `factions`

`id`, `book_id`, `name`, `goal`, `power`, `status`

## `relationships`

`id`, `book_id`, `from_id`, `to_id`, `type`, `trust`, `notes`, `updated_ch`

## `timeline_events`

`id`, `book_id`, `chapter`, `when`, `fact`, `sources`, `characters`

## `foreshadows`

`id` (`FS-001`…), `book_id`, `summary`, `plant_ch`, `expect_payoff_ch`, `status` (`open|advanced|paid|dropped`), `urgency`

## `resources`

`id`, `book_id`, `kind` (`goldfinger|item|power`), `name`, `cost`, `limits`, `last_uses`, `notes`

## `chapter_contracts`

`id`, `book_id`, `chapter`, `unit_id`, `purpose`, `beats`, `forbidden`, `hook`, `pov`, `status`, `file`

Mirror/index only. Authoritative contract body is always
`novel/<book-id>/chapters/chNNN-contract.yaml`. Prefer `file: chapters/chNNN-contract.yaml`.

## `continuity_issues`

`id`, `book_id`, `chapter`, `severity` (`P0|P1`), `lens`, `quote`, `status` (`open|fixed`)

## `reader_debts`

`id`, `book_id`, `chapter`, `hook_summary`, `hook_type`, `status` (`open|paid|downgraded`), `payoff_ch`, `urgency`

Track open chapter-end hooks per KB「追读力」；开放债务建议 ≤5。

## Query habits

- Prefer `table_query` with `book_id` + chapter/relevance filters.  
- Never load all rows into the draft prompt — select involved ids from the contract.
