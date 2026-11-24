// Package store owns the workspace's SQLite database: opening a workspace,
// applying schema migrations, and the repositories for ledgers, records,
// categories/tags, templates, events, diary, notes, todo and stock data.
//
// Contract:
//
//   - a workspace is a directory containing transactions.db; the directory must
//     already exist, and a missing database is created at the current schema;
//   - schema versions are contiguous from 1: 0001_init.sql is the 0.1.0 schema and
//     later features append their own file. app_meta.schema_version names the
//     version the database holds, and a database written by a newer build is
//     refused;
//   - opening runs PRAGMA quick_check only; integrity_check is reserved for the
//     explicit backup and restore actions;
//   - amounts are integer cents (columns ending _cents), times are Unix seconds
//     (columns ending _at), dates are local "YYYY-MM-DD" (columns ending _date);
//   - there are no foreign keys; deletes are explicit transactions in the store.
package store
