// Package store will own the workspace's SQLite database: opening a workspace,
// applying the schema migrations, and the DAOs that read and write each table.
//
// It is a documented seam for now, not an implementation. The contract it must
// keep is the old project's electron/db:
//
//   - the database is transactions.db inside the user-chosen workspace
//     directory; images live under data/assets in the same workspace;
//   - schema versions are recorded in app_meta.schema_version and old
//     workspaces are upgraded once, never silently recreated;
//   - opening a workspace runs PRAGMA quick_check only; integrity_check is
//     reserved for the explicit backup and restore actions;
//   - SQLite units are the domain's: amounts in integer cents (columns ending
//     _cents), stock prices in integer milli (columns ending _milli), times as
//     Unix seconds (columns ending _at), dates as local "YYYY-MM-DD" (columns
//     ending _date).
//
// The schema and its later migrations are ported together with the first
// screen that needs them (记账), so this package starts with a blank file
// rather than a half-copied schema that would drift from the old database.
package store
