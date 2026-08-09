package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sponsor errors.
var (
	ErrInvalidSponsor  = errors.New("sponsor name is required")
	ErrSponsorNotFound = errors.New("sponsor not found")
)

// Sponsor is a brand the operator works with (SPEC §5.6).
type Sponsor struct {
	ID        int64
	Name      string
	Contact   string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateSponsor inserts a sponsor (name required).
func CreateSponsor(ctx context.Context, db *sql.DB, name, contact, notes string) (Sponsor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Sponsor{}, ErrInvalidSponsor
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO sponsors (name, contact, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		name, strings.TrimSpace(contact), strings.TrimSpace(notes), ts, ts)
	if err != nil {
		return Sponsor{}, fmt.Errorf("insert sponsor: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Sponsor{}, fmt.Errorf("sponsor last insert id: %w", err)
	}
	return Sponsor{ID: id, Name: name, Contact: strings.TrimSpace(contact), Notes: strings.TrimSpace(notes), CreatedAt: now, UpdatedAt: now}, nil
}

// ListSponsors returns all sponsors ordered case-insensitively by name.
func ListSponsors(ctx context.Context, db *sql.DB) ([]Sponsor, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, contact, notes, created_at, updated_at FROM sponsors ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query sponsors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Sponsor
	for rows.Next() {
		s, err := scanSponsor(rows)
		if err != nil {
			return nil, fmt.Errorf("scan sponsor: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sponsors: %w", err)
	}
	return out, nil
}

// GetSponsor returns one sponsor, or ErrSponsorNotFound.
func GetSponsor(ctx context.Context, db *sql.DB, id int64) (Sponsor, error) {
	s, err := scanSponsor(db.QueryRowContext(ctx,
		`SELECT id, name, contact, notes, created_at, updated_at FROM sponsors WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Sponsor{}, ErrSponsorNotFound
	}
	if err != nil {
		return Sponsor{}, fmt.Errorf("get sponsor: %w", err)
	}
	return s, nil
}

// UpdateSponsor updates a sponsor's fields (name required).
func UpdateSponsor(ctx context.Context, db *sql.DB, id int64, name, contact, notes string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidSponsor
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE sponsors SET name = ?, contact = ?, notes = ?, updated_at = ? WHERE id = ?`,
		name, strings.TrimSpace(contact), strings.TrimSpace(notes), ts, id)
	if err != nil {
		return fmt.Errorf("update sponsor: %w", err)
	}
	return checkAffected(res, ErrSponsorNotFound)
}

// DeleteSponsor removes a sponsor (and, by cascade, its campaigns).
func DeleteSponsor(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM sponsors WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete sponsor: %w", err)
	}
	return checkAffected(res, ErrSponsorNotFound)
}

func scanSponsor(sc rowScanner) (Sponsor, error) {
	var (
		s            Sponsor
		created, upd string
	)
	if err := sc.Scan(&s.ID, &s.Name, &s.Contact, &s.Notes, &created, &upd); err != nil {
		return Sponsor{}, err
	}
	s.CreatedAt = parseTS(created)
	s.UpdatedAt = parseTS(upd)
	return s, nil
}
