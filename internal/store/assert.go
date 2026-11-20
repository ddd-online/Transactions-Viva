package store

import "transactions/internal/repository"

// The workspace is the accounting data source. Keeping the assertion here
// makes an interface change fail at compile time in the implementation.
var _ repository.Repository = (*Workspace)(nil)
