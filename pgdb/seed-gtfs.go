package pgdb

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const gtfsDir = "/home/archusr/Data/Development/solid-parakeet-go/pgdb/gtfs_data" // Folder containing unzipped .txt files

type GTFSTable struct {
	FileName  string
	TableName string
	Required  bool
}

// ingestion sequence ordered by relational dependencies
var tables = []GTFSTable{
	{FileName: "agency.txt", TableName: "agency", Required: false},
	{FileName: "calendar.txt", TableName: "calendar", Required: false},
	{FileName: "calendar_dates.txt", TableName: "calendar_dates", Required: false},
	{FileName: "routes.txt", TableName: "routes", Required: true},
	{FileName: "shapes.txt", TableName: "shapes", Required: false},
	{FileName: "stops.txt", TableName: "stops", Required: true},
	{FileName: "trips.txt", TableName: "trips", Required: true},
	{FileName: "stop_times.txt", TableName: "stop_times", Required: true},
}

func ConsumeGTFS(ctx context.Context, pool *pgxpool.Pool) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Fatalf("Failed to acquire connection from pool: %v", err)
	}
	defer conn.Release()

	// Use the underlying *pgx.Conn from the acquired pool connection
	pgxConn := conn.Conn()

	log.Println("Initializing GTFS database schema...")
	if err := initSchema(ctx, pgxConn); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	log.Println("Ingesting GTFS files...")
	startTime := time.Now()

	for _, t := range tables {
		filePath := filepath.Join(gtfsDir, t.FileName)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if t.Required {
				log.Fatalf("Required file missing: %s", t.FileName)
			}
			log.Printf("Skipping optional file: %s (not found)", t.FileName)
			continue
		}

		// Check if table already has rows
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s LIMIT 1);", t.TableName)
		if err := pgxConn.QueryRow(ctx, query).Scan(&exists); err == nil && exists {
			log.Printf("Skipping %s: table %s already populated", t.FileName, t.TableName)
			continue
		}

		if err := ingestGTFSFile(ctx, pgxConn, filePath, t.TableName); err != nil {
			log.Fatalf("Error loading %s: %v", t.FileName, err)
		}
	}

	log.Println("Creating post-ingestion indexes...")

	if err := createIndexes(ctx, pgxConn); err != nil {
		log.Fatalf("Failed to create indexes: %v", err)
	}

	log.Printf("All GTFS data loaded successfully in %s\n", time.Since(startTime).Round(time.Millisecond))
}

func initSchema(ctx context.Context, conn *pgx.Conn) error {
	schemaSQL := `
	CREATE TABLE IF NOT EXISTS agency (
		agency_id       TEXT PRIMARY KEY,
		agency_name     TEXT NOT NULL,
		agency_url      TEXT NOT NULL,
		agency_timezone TEXT NOT NULL,
		agency_lang     TEXT,
		agency_phone    TEXT,
		agency_fare_url TEXT,
		agency_email    TEXT
	);

	CREATE TABLE IF NOT EXISTS calendar (
		service_id TEXT PRIMARY KEY,
		monday     SMALLINT NOT NULL,
		tuesday    SMALLINT NOT NULL,
		wednesday  SMALLINT NOT NULL,
		thursday   SMALLINT NOT NULL,
		friday     SMALLINT NOT NULL,
		saturday   SMALLINT NOT NULL,
		sunday     SMALLINT NOT NULL,
		start_date CHAR(8) NOT NULL,
		end_date   CHAR(8) NOT NULL
	);

	CREATE TABLE IF NOT EXISTS calendar_dates (
		service_id     TEXT NOT NULL,
		date           CHAR(8) NOT NULL,
		exception_type SMALLINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS routes (
		route_id         TEXT PRIMARY KEY,
		agency_id        TEXT,
		route_short_name TEXT,
		route_long_name  TEXT,
		route_desc       TEXT,
		route_type       SMALLINT NOT NULL,
		route_url        TEXT,
		route_color      VARCHAR(6),
		route_text_color VARCHAR(6),
		route_sort_order INTEGER
	);

	CREATE TABLE IF NOT EXISTS shapes (
		shape_id            TEXT NOT NULL,
		shape_pt_lat        DOUBLE PRECISION NOT NULL,
		shape_pt_lon        DOUBLE PRECISION NOT NULL,
		shape_pt_sequence   INTEGER NOT NULL,
		shape_dist_traveled DOUBLE PRECISION
	);

	CREATE TABLE IF NOT EXISTS stops (
		stop_id             TEXT PRIMARY KEY,
		stop_code           TEXT,
		stop_name           TEXT NOT NULL,
		stop_desc           TEXT,
		stop_lat            DOUBLE PRECISION NOT NULL,
		stop_lon            DOUBLE PRECISION NOT NULL,
		zone_id             TEXT,
		stop_url            TEXT,
		location_type       SMALLINT DEFAULT 0,
		parent_station      TEXT,
		stop_timezone       TEXT,
		wheelchair_boarding SMALLINT DEFAULT 0,
		platform_code       TEXT
	);

	CREATE TABLE IF NOT EXISTS trips (
		route_id              TEXT NOT NULL,
		service_id            TEXT NOT NULL,
		trip_id               TEXT PRIMARY KEY,
		trip_headsign         TEXT,
		trip_short_name       TEXT,
		direction_id          SMALLINT,
		block_id              TEXT,
		shape_id              TEXT,
		wheelchair_accessible SMALLINT DEFAULT 0,
		bikes_allowed         SMALLINT DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS stop_times (
		trip_id             TEXT NOT NULL,
		arrival_time        TEXT,
		departure_time      TEXT,
		stop_id             TEXT NOT NULL,
		stop_sequence       INTEGER NOT NULL,
		stop_headsign       TEXT,
		pickup_type         SMALLINT DEFAULT 0,
		drop_off_type       SMALLINT DEFAULT 0,
		shape_dist_traveled DOUBLE PRECISION,
		timepoint           SMALLINT DEFAULT 1
	);
	`
	_, err := conn.Exec(ctx, schemaSQL)
	return err
}

func ingestGTFSFile(ctx context.Context, conn *pgx.Conn, filePath, tableName string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Parse header to dynamically map columns defined in this feed
	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to parse CSV header: %w", err)
	}

	var sanitizedCols []string
	for i, col := range header {
		clean := strings.TrimSpace(col)
		if i == 0 {
			clean = strings.TrimPrefix(clean, "\ufeff") // Strip potential UTF-8 BOM
		}
		sanitizedCols = append(sanitizedCols, clean)
	}

	// Reset file offset to beginning so COPY FROM STDIN processes the entire CSV
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to reset file pointer: %w", err)
	}

	copySQL := fmt.Sprintf(
		"COPY %s (%s) FROM STDIN WITH (FORMAT csv, HEADER true)",
		tableName,
		strings.Join(sanitizedCols, ", "),
	)

	start := time.Now()
	tag, err := conn.PgConn().CopyFrom(ctx, file, copySQL)
	if err != nil {
		return fmt.Errorf("copy failed on %s: %w", tableName, err)
	}

	log.Printf(" -> [%s] Imported %d rows in %s", tableName, tag.RowsAffected(), time.Since(start).Round(time.Millisecond))
	return nil
}

func createIndexes(ctx context.Context, conn *pgx.Conn) error {
	indexSQL := `
	CREATE INDEX IF NOT EXISTS idx_stop_times_trip_seq ON stop_times (trip_id, stop_sequence);
	CREATE INDEX IF NOT EXISTS idx_stop_times_stop_id ON stop_times (stop_id);
	CREATE INDEX IF NOT EXISTS idx_trips_route_id ON trips (route_id);
	CREATE INDEX IF NOT EXISTS idx_trips_service_id ON trips (service_id);
	CREATE INDEX IF NOT EXISTS idx_shapes_shape_id ON shapes (shape_id, shape_pt_sequence);
	CREATE INDEX IF NOT EXISTS idx_stops_lat_lon ON stops (stop_lat, stop_lon);
	`
	_, err := conn.Exec(ctx, indexSQL)
	return err
}
