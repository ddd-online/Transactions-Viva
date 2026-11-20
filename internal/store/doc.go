// Package store owns the workspace's SQLite database: opening a workspace,
// applying schema migrations, and the repositories for ledgers, records,
// categories/tags and templates.
//
// Contract:
//
//   - a workspace is a directory containing transactions.db; the directory must
//     already exist, and a missing database is created and migrated to v1;
//   - schema versions live in app_meta.schema_version; versions are contiguous
//     from 1, upgrades back up the database to transactions.db.bak first, and a
//     database written by a newer build is refused;
//   - opening runs PRAGMA quick_check only; integrity_check is reserved for the
//     explicit backup and restore actions;
//   - the accounting tables are column-for-column compatible with the old
//     project's schema, so internal/store/import_old.go can copy old rows
//     without translation. The old stock/chart/event/diary/todo tables are not
//     created here;
//   - amounts are integer cents (columns ending _cents), times are Unix seconds
//     (columns ending _at), dates are local "YYYY-MM-DD" (columns ending _date);
//   - there are no foreign keys, as in the old schema; deletes are explicit
//     transactions in the store.
//
// The old application's database (schema version 12 and stock tables) is not
// opened directly: it is recognized and reported, and the import tool writes a
// new workspace instead. This keeps the new schema's version history
// independent of the old one.
package store
