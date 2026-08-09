package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// errOrderedNotFound is the internal sentinel from moveOrdered; callers translate
// it to their table-specific not-found error.
var errOrderedNotFound = errors.New("ordered row not found")

// orderedTables allowlists the tables moveOrdered may touch. Table names are
// interpolated into SQL, so they must come only from this set — never user input.
var orderedTables = map[string]bool{
	"outline_segments": true,
	"shots":            true,
}

// moveOrdered swaps a row's position with its neighbor in the given direction
// ("up"/"down") within one transaction, scoped to a content item. Moving past an
// edge is a no-op. Returns ErrInvalidMove for a bad direction and
// errOrderedNotFound if the row is absent.
func moveOrdered(ctx context.Context, db *sql.DB, table string, id, contentItemID int64, dir string) error {
	if dir != "up" && dir != "down" {
		return ErrInvalidMove
	}
	if !orderedTables[table] {
		return fmt.Errorf("moveOrdered: unknown table %q", table)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin move tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var pos int
	err = tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT position FROM %s WHERE id = ? AND content_item_id = ?`, table),
		id, contentItemID).Scan(&pos)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return errOrderedNotFound
	case err != nil:
		return fmt.Errorf("load position: %w", err)
	}

	cmp, order := "<", "DESC" // "up": the largest position still below this one
	if dir == "down" {
		cmp, order = ">", "ASC"
	}
	var (
		neighborID  int64
		neighborPos int
	)
	err = tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id, position FROM %s WHERE content_item_id = ? AND position %s ? ORDER BY position %s LIMIT 1`, table, cmp, order),
		contentItemID, pos).Scan(&neighborID, &neighborPos)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // at the edge; nothing to do
	}
	if err != nil {
		return fmt.Errorf("load neighbor: %w", err)
	}

	stmt := fmt.Sprintf(`UPDATE %s SET position = ? WHERE id = ?`, table)
	if _, err := tx.ExecContext(ctx, stmt, neighborPos, id); err != nil {
		return fmt.Errorf("move row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, stmt, pos, neighborID); err != nil {
		return fmt.Errorf("move neighbor: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit move: %w", err)
	}
	return nil
}
