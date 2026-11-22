// Package store owns the workspace's SQLite database: opening a workspace,
// applying schema migrations, and the repositories for ledgers, records,
// categories/tags, templates, events, diary, todo and stock data.
//
// Contract:
//
//   - a workspace is a directory containing transactions.db; the directory must
//     already exist, and a missing database is created at schema v1;
//   - the current release has one schema version, v1, in app_meta.schema_version.
//     A database written by a newer build is refused;
//   - opening runs PRAGMA quick_check only; integrity_check is reserved for the
//     explicit backup and restore actions;
//   - amounts are integer cents (columns ending _cents), times are Unix seconds
//     (columns ending _at), dates are local "YYYY-MM-DD" (columns ending _date);
//   - there are no foreign keys; deletes are explicit transactions in the store.
package store
