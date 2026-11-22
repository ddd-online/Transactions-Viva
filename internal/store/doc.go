// Package store owns the workspace's SQLite database: opening a workspace,
// applying schema migrations, and the repositories for ledgers, records,
// categories/tags and templates.
//
// Contract:
//
//   - a workspace is a directory containing transactions.db; the directory must
//     already exist, and a missing database is created at schema v1;
//   - the 0.1.0 release has one schema version, v1, in app_meta.schema_version.
//     There are no incremental migrations; a pre-0.1.0 development database
//     (version 2–7) that already carries every v1 table is adopted as v1 after
//     a transactions.db.bak backup. A database written by a newer build is
//     refused;
//   - opening runs PRAGMA quick_check only; integrity_check is reserved for the
//     explicit backup and restore actions;
//   - the accounting tables are column-for-column compatible with the old
//     project's schema, so internal/store/import_old.go can copy old rows
//     without translation; the old database's chart, event, stock and todo
//     tables are copied into the v1 tables too;
//   - amounts are integer cents (columns ending _cents), times are Unix seconds
//     (columns ending _at), dates are local "YYYY-MM-DD" (columns ending _date);
//   - there are no foreign keys, as in the old schema; deletes are explicit
//     transactions in the store.
//
// The old application's database (schema version 12 and stock tables) is not
// opened directly: it is recognized and reported, and the import tool writes a
// new v1 workspace instead.
package store
