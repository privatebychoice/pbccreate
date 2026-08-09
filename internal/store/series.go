package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Series errors.
var (
	ErrInvalidSeries     = errors.New("series name is required")
	ErrSeriesNotFound    = errors.New("series not found")
	ErrEpisodeNotFound   = errors.New("series episode not found")
	ErrEpisodeExists     = errors.New("content item is already in this series")
	ErrInvalidEpisode    = errors.New("a content item is required")
	ErrItemChannelDiffer = errors.New("content item belongs to a different channel")
)

// Series groups content items into an ordered playlist/arc (SPEC §5.14).
type Series struct {
	ID          int64
	ChannelID   int64
	ChannelName string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SeriesSummary pairs a series with episode counts for the list view.
type SeriesSummary struct {
	Series
	EpisodeCount int
	DoneCount    int // episodes whose content item is published
}

// Episode is one ordered entry in a series, joined with its content item.
type Episode struct {
	ID            int64 // series_items.id
	SeriesID      int64
	ContentItemID int64
	Position      int
	ArcNotes      string
	Title         string
	Status        string
}

// CreateSeries inserts a series (name required) and returns it.
func CreateSeries(ctx context.Context, db *sql.DB, channelID int64, name, description string) (Series, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Series{}, ErrInvalidSeries
	}
	now := time.Now().UTC().Truncate(time.Second)
	ts := now.Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`INSERT INTO series (channel_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		channelID, name, strings.TrimSpace(description), ts, ts)
	if err != nil {
		return Series{}, fmt.Errorf("insert series: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Series{}, fmt.Errorf("series last insert id: %w", err)
	}
	return GetSeries(ctx, db, id)
}

// GetSeries returns one series with its channel name, or ErrSeriesNotFound.
func GetSeries(ctx context.Context, db *sql.DB, id int64) (Series, error) {
	var (
		s            Series
		created, upd string
	)
	err := db.QueryRowContext(ctx, `
		SELECT s.id, s.channel_id, COALESCE(c.name, ''), s.name, s.description, s.created_at, s.updated_at
		FROM series s LEFT JOIN channels c ON c.id = s.channel_id
		WHERE s.id = ?`, id).Scan(&s.ID, &s.ChannelID, &s.ChannelName, &s.Name, &s.Description, &created, &upd)
	if errors.Is(err, sql.ErrNoRows) {
		return Series{}, ErrSeriesNotFound
	}
	if err != nil {
		return Series{}, fmt.Errorf("get series: %w", err)
	}
	s.CreatedAt = parseTS(created)
	s.UpdatedAt = parseTS(upd)
	return s, nil
}

// ListSeries returns all series with episode/done counts, ordered by channel
// then name.
func ListSeries(ctx context.Context, db *sql.DB) ([]SeriesSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT s.id, s.channel_id, COALESCE(c.name, ''), s.name, s.description, s.created_at, s.updated_at,
			(SELECT COUNT(*) FROM series_items si WHERE si.series_id = s.id),
			(SELECT COUNT(*) FROM series_items si JOIN content_items ci ON ci.id = si.content_item_id
				WHERE si.series_id = s.id AND ci.status = 'published')
		FROM series s LEFT JOIN channels c ON c.id = s.channel_id
		ORDER BY c.name COLLATE NOCASE, s.name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query series: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SeriesSummary
	for rows.Next() {
		var (
			ss           SeriesSummary
			created, upd string
		)
		if err := rows.Scan(&ss.ID, &ss.ChannelID, &ss.ChannelName, &ss.Name, &ss.Description,
			&created, &upd, &ss.EpisodeCount, &ss.DoneCount); err != nil {
			return nil, fmt.Errorf("scan series: %w", err)
		}
		ss.CreatedAt = parseTS(created)
		ss.UpdatedAt = parseTS(upd)
		out = append(out, ss)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate series: %w", err)
	}
	return out, nil
}

// UpdateSeries updates a series' name/description (name required).
func UpdateSeries(ctx context.Context, db *sql.DB, id int64, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrInvalidSeries
	}
	ts := time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)
	res, err := db.ExecContext(ctx,
		`UPDATE series SET name = ?, description = ?, updated_at = ? WHERE id = ?`,
		name, strings.TrimSpace(description), ts, id)
	if err != nil {
		return fmt.Errorf("update series: %w", err)
	}
	return checkAffected(res, ErrSeriesNotFound)
}

// DeleteSeries removes a series (and its episode links, by cascade).
func DeleteSeries(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM series WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete series: %w", err)
	}
	return checkAffected(res, ErrSeriesNotFound)
}

// ListEpisodes returns a series' episodes in order, joined with content items.
func ListEpisodes(ctx context.Context, db *sql.DB, seriesID int64) ([]Episode, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT si.id, si.series_id, si.content_item_id, si.position, si.arc_notes, ci.title, ci.status
		FROM series_items si
		JOIN content_items ci ON ci.id = si.content_item_id
		WHERE si.series_id = ?
		ORDER BY si.position, si.id`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("query episodes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Episode
	for rows.Next() {
		var e Episode
		if err := rows.Scan(&e.ID, &e.SeriesID, &e.ContentItemID, &e.Position, &e.ArcNotes, &e.Title, &e.Status); err != nil {
			return nil, fmt.Errorf("scan episode: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate episodes: %w", err)
	}
	return out, nil
}

// AddEpisode appends a content item to a series (must belong to the same
// channel; must not already be in the series). Position is next-in-line.
func AddEpisode(ctx context.Context, db *sql.DB, seriesID, contentItemID int64) error {
	if contentItemID <= 0 {
		return ErrInvalidEpisode
	}
	series, err := GetSeries(ctx, db, seriesID)
	if err != nil {
		return err
	}
	item, err := GetContentItem(ctx, db, contentItemID)
	if err != nil {
		return err
	}
	if item.ChannelID != series.ChannelID {
		return ErrItemChannelDiffer
	}

	// Already in the series? (pre-check, matching the placement store's approach).
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM series_items WHERE series_id = ? AND content_item_id = ?`,
		seriesID, contentItemID).Scan(&n); err != nil {
		return fmt.Errorf("check existing episode: %w", err)
	}
	if n > 0 {
		return ErrEpisodeExists
	}

	var next int
	if err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), 0) + 1 FROM series_items WHERE series_id = ?`, seriesID).Scan(&next); err != nil {
		return fmt.Errorf("next episode position: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO series_items (series_id, content_item_id, position) VALUES (?, ?, ?)`,
		seriesID, contentItemID, next); err != nil {
		return fmt.Errorf("insert episode: %w", err)
	}
	return nil
}

// UpdateEpisodeArc sets an episode's continuity/arc notes.
func UpdateEpisodeArc(ctx context.Context, db *sql.DB, episodeID, seriesID int64, notes string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE series_items SET arc_notes = ? WHERE id = ? AND series_id = ?`,
		strings.TrimSpace(notes), episodeID, seriesID)
	if err != nil {
		return fmt.Errorf("update episode arc: %w", err)
	}
	return checkAffected(res, ErrEpisodeNotFound)
}

// RemoveEpisode removes a content item from a series (the item itself is kept).
func RemoveEpisode(ctx context.Context, db *sql.DB, episodeID, seriesID int64) error {
	res, err := db.ExecContext(ctx,
		`DELETE FROM series_items WHERE id = ? AND series_id = ?`, episodeID, seriesID)
	if err != nil {
		return fmt.Errorf("remove episode: %w", err)
	}
	return checkAffected(res, ErrEpisodeNotFound)
}

// MoveEpisode reorders an episode up or down within its series.
func MoveEpisode(ctx context.Context, db *sql.DB, episodeID, seriesID int64, dir string) error {
	err := moveOrdered(ctx, db, "series_items", episodeID, seriesID, dir)
	if errors.Is(err, errOrderedNotFound) {
		return ErrEpisodeNotFound
	}
	return err
}
