package repository

import (
	"context"

	"transactions/internal/domain"
)

// NotesRepository is the workspace data the 笔记 section uses. Notes belong to
// the workspace, not to a ledger: one tree, shared by the whole workspace.
//
// A folder may hold folders and notes, a note is always a leaf, and the root
// holds folders only. Block order is the order of the blocks slice.
type NotesRepository interface {
	// NoteTree returns every node; the page assembles the tree from ParentID
	// and SortOrder.
	NoteTree(ctx context.Context) ([]domain.NoteNode, error)
	// Note returns one note with its blocks, top to bottom.
	Note(ctx context.Context, id string) (domain.Note, error)
	// CreateNoteNode appends a folder or a note to parentID, or to the root
	// when parentID is empty (folders only there).
	CreateNoteNode(ctx context.Context, parentID, kind, title string) (domain.NoteNode, error)
	// RenameNoteNode replaces a node's title; an empty title shows the default.
	RenameNoteNode(ctx context.Context, id, title string) error
	// DeleteNoteNode removes a node and its whole subtree, returning the notes
	// and image paths deleted so the caller can clean the files.
	DeleteNoteNode(ctx context.Context, id string) (domain.NoteDeleted, error)
	// MoveNoteNode places a node under parentID at position among its new
	// siblings; a negative position appends. A folder cannot move into itself,
	// and a note cannot move to the root.
	MoveNoteNode(ctx context.Context, id, parentID string, position int) error
	// ReorderNoteNodes rewrites one folder's child order to ids.
	ReorderNoteNodes(ctx context.Context, parentID string, ids []string) error
	// SetNoteNodeExpanded stores a folder's fold state.
	SetNoteNodeExpanded(ctx context.Context, id string, expanded bool) error
	// SaveNote writes a note's title and blocks, assigning ids to new blocks.
	SaveNote(ctx context.Context, note domain.Note) (domain.Note, error)
	// LastNoteID is the note opened last, or empty.
	LastNoteID(ctx context.Context) (string, error)
	SetLastNoteID(ctx context.Context, id string) error
}
