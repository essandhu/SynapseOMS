package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// VenueRow represents a row in the venues table.
type VenueRow struct {
	ID            string
	Type          string
	Name          string
	Status        string
	ConfigJSON    json.RawMessage
	LastHeartbeat *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// UpsertVenue inserts or updates a venue using INSERT ON CONFLICT.
func (s *PostgresStore) UpsertVenue(ctx context.Context, v *VenueRow) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO venues (
			id, type, name, status, config_json,
			last_heartbeat, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8
		)
		ON CONFLICT (id) DO UPDATE SET
			type = EXCLUDED.type,
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			config_json = EXCLUDED.config_json,
			last_heartbeat = EXCLUDED.last_heartbeat,
			updated_at = EXCLUDED.updated_at`,
		v.ID,
		v.Type,
		v.Name,
		v.Status,
		v.ConfigJSON,
		v.LastHeartbeat,
		v.CreatedAt,
		v.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting venue: %w", err)
	}
	return nil
}

// GetVenue retrieves a single venue by its ID.
func (s *PostgresStore) GetVenue(ctx context.Context, id string) (*VenueRow, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, type, name, status, config_json,
			last_heartbeat, created_at, updated_at
		FROM venues
		WHERE id = $1`, id)

	return scanVenue(row)
}

// ListVenues retrieves all venues.
func (s *PostgresStore) ListVenues(ctx context.Context) ([]VenueRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, type, name, status, config_json,
			last_heartbeat, created_at, updated_at
		FROM venues
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing venues: %w", err)
	}
	defer rows.Close()

	var venues []VenueRow
	for rows.Next() {
		v, err := scanVenueFromRows(rows)
		if err != nil {
			return nil, err
		}
		venues = append(venues, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating venues: %w", err)
	}

	return venues, nil
}

// UpdateVenueStatus updates a venue's status and last heartbeat timestamp.
func (s *PostgresStore) UpdateVenueStatus(ctx context.Context, id, status string, lastHeartbeat time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE venues SET
			status = $1,
			last_heartbeat = $2,
			updated_at = NOW()
		WHERE id = $3`,
		status,
		lastHeartbeat,
		id,
	)
	if err != nil {
		return fmt.Errorf("updating venue status: %w", err)
	}
	return nil
}

// DeleteVenue removes a venue by its ID.
func (s *PostgresStore) DeleteVenue(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM venues WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting venue: %w", err)
	}
	return nil
}

// scanVenue scans a single venue from a pgx.Row.
func scanVenue(row pgx.Row) (*VenueRow, error) {
	var v VenueRow

	err := row.Scan(
		&v.ID, &v.Type, &v.Name, &v.Status, &v.ConfigJSON,
		&v.LastHeartbeat, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning venue: %w", err)
	}

	return &v, nil
}

// scanVenueFromRows scans a venue from pgx.Rows (same columns as scanVenue).
func scanVenueFromRows(rows pgx.Rows) (*VenueRow, error) {
	var v VenueRow

	err := rows.Scan(
		&v.ID, &v.Type, &v.Name, &v.Status, &v.ConfigJSON,
		&v.LastHeartbeat, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning venue row: %w", err)
	}

	return &v, nil
}
