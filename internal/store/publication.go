package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Publication errors and vocabulary.
var (
	ErrInvalidPublication  = errors.New("publication requires a platform")
	ErrPublicationNotFound = errors.New("publication not found")

	// Visibilities is the publish-visibility vocabulary; "" means unset.
	Visibilities = []string{"public", "unlisted", "private", "scheduled"}
)

// Publication is a per-platform publication record for a content item (SPEC
// §5.12). A single item may have several (one per platform). Metrics are entered
// by hand; pbccreate makes no network calls.
type Publication struct {
	ID             int64
	ContentItemID  int64
	Platform       string
	PublishedTitle string
	ExternalID     string
	URL            string
	OutputFile     string
	PostedOn       string // YYYY-MM-DD or ""
	Visibility     string
	TagsSnapshot   string
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// normVisibility keeps a valid visibility or clears it to "" (unset).
func normVisibility(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || slices.Contains(Visibilities, s) {
		return s
	}
	return ""
}

const publicationColumns = `id, content_item_id, platform, published_title, external_id,
	url, output_file, posted_on, visibility, tags_snapshot, notes, created_at, updated_at`

// clean trims and normalizes a publication's fields in place (platform is
// validated by the caller).
func (p *Publication) clean() {
	p.Platform = strings.TrimSpace(p.Platform)
	p.PublishedTitle = strings.TrimSpace(p.PublishedTitle)
	p.ExternalID = strings.TrimSpace(p.ExternalID)
	p.URL = strings.TrimSpace(p.URL)
	p.OutputFile = strings.TrimSpace(p.OutputFile)
	p.PostedOn = strings.TrimSpace(p.PostedOn)
	p.Visibility = normVisibility(p.Visibility)
	p.TagsSnapshot = strings.TrimSpace(p.TagsSnapshot)
	p.Notes = strings.TrimSpace(p.Notes)
}

// CreatePublication inserts a publication (platform required) and returns it.
func CreatePublication(ctx context.Context, db *sql.DB, p Publication) (Publication, error) {
	p.clean()
	if p.Platform == "" {
		return Publication{}, ErrInvalidPublication
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		INSERT INTO publications
			(content_item_id, platform, published_title, external_id, url, output_file, posted_on, visibility, tags_snapshot, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ContentItemID, p.Platform, p.PublishedTitle, p.ExternalID, p.URL, p.OutputFile,
		p.PostedOn, p.Visibility, p.TagsSnapshot, p.Notes, ts, ts)
	if err != nil {
		return Publication{}, fmt.Errorf("insert publication: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Publication{}, fmt.Errorf("publication last insert id: %w", err)
	}
	p.ID = id
	p.CreatedAt, p.UpdatedAt = now, now
	return p, nil
}

// ListPublications returns a content item's publications, newest first.
func ListPublications(ctx context.Context, db *sql.DB, contentItemID int64) ([]Publication, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+publicationColumns+` FROM publications WHERE content_item_id = ? ORDER BY id DESC`, contentItemID)
	if err != nil {
		return nil, fmt.Errorf("query publications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Publication
	for rows.Next() {
		p, err := scanPublication(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate publications: %w", err)
	}
	return out, nil
}

// GetPublication returns one publication scoped to its content item.
func GetPublication(ctx context.Context, db *sql.DB, id, contentItemID int64) (Publication, error) {
	p, err := scanPublication(db.QueryRowContext(ctx,
		`SELECT `+publicationColumns+` FROM publications WHERE id = ? AND content_item_id = ?`, id, contentItemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Publication{}, ErrPublicationNotFound
	}
	if err != nil {
		return Publication{}, fmt.Errorf("get publication: %w", err)
	}
	return p, nil
}

// UpdatePublication updates a publication scoped to its content item (platform
// required).
func UpdatePublication(ctx context.Context, db *sql.DB, p Publication) error {
	p.clean()
	if p.Platform == "" {
		return ErrInvalidPublication
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx, `
		UPDATE publications SET
			platform = ?, published_title = ?, external_id = ?, url = ?, output_file = ?,
			posted_on = ?, visibility = ?, tags_snapshot = ?, notes = ?, updated_at = ?
		WHERE id = ? AND content_item_id = ?`,
		p.Platform, p.PublishedTitle, p.ExternalID, p.URL, p.OutputFile,
		p.PostedOn, p.Visibility, p.TagsSnapshot, p.Notes, ts, p.ID, p.ContentItemID)
	if err != nil {
		return fmt.Errorf("update publication: %w", err)
	}
	return checkAffected(res, ErrPublicationNotFound)
}

// DeletePublication removes a publication scoped to its content item.
func DeletePublication(ctx context.Context, db *sql.DB, id, contentItemID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM publications WHERE id = ? AND content_item_id = ?`, id, contentItemID)
	if err != nil {
		return fmt.Errorf("delete publication: %w", err)
	}
	return checkAffected(res, ErrPublicationNotFound)
}

func scanPublication(sc rowScanner) (Publication, error) {
	var (
		p            Publication
		created, upd string
	)
	if err := sc.Scan(&p.ID, &p.ContentItemID, &p.Platform, &p.PublishedTitle, &p.ExternalID,
		&p.URL, &p.OutputFile, &p.PostedOn, &p.Visibility, &p.TagsSnapshot, &p.Notes, &created, &upd); err != nil {
		return Publication{}, err
	}
	p.CreatedAt = parseTS(created)
	p.UpdatedAt = parseTS(upd)
	return p, nil
}
