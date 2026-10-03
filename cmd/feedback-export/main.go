// feedback-export is an operator-only reader. It requires database credentials;
// publishable project keys cannot list reports or retrieve their images.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
	"path/filepath"
	"time"
)

type report struct {
	ID              string    `json:"id"`
	Resource        string    `json:"resource"`
	Kind            string    `json:"kind"`
	Body            string    `json:"body"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	HasAttachment   bool      `json:"has_attachment"`
	AttachmentCount int       `json:"attachment_count"`
}

func main() {
	project := flag.String("project", "", "Project slug (required)")
	id := flag.String("id", "", "Report UUID to export; omit to list the newest reports")
	output := flag.String("output", "", "New private directory for report.json and image.jpg")
	limit := flag.Int("limit", 20, "Number of reports to list (1-100)")
	flag.Parse()
	if *project == "" || *limit < 1 || *limit > 100 || (*id != "" && *output == "") {
		log.Fatal("provide --project, a limit of 1-100, and --output when exporting --id")
	}
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, database)
	if err != nil {
		log.Fatal("database configuration failed")
	}
	defer pool.Close()
	if *id == "" {
		rows, err := pool.Query(ctx, `SELECT f.id::text,f.resource,f.kind::text,f.body,f.status::text,f.created_at,CASE WHEN f.created_at>=now()-interval '30 days' THEN (CASE WHEN f.attachment IS NOT NULL THEN 1 ELSE 0 END)+cardinality(f.extra_attachments) ELSE 0 END FROM feedback f JOIN projects p ON p.id=f.project_id WHERE p.slug=$1 ORDER BY f.created_at DESC LIMIT $2`, *project, *limit)
		if err != nil {
			log.Fatal("report lookup failed")
		}
		defer rows.Close()
		reports := []report{}
		for rows.Next() {
			var r report
			if rows.Scan(&r.ID, &r.Resource, &r.Kind, &r.Body, &r.Status, &r.CreatedAt, &r.AttachmentCount) != nil {
				log.Fatal("report read failed")
			}
			r.HasAttachment = r.AttachmentCount > 0
			reports = append(reports, r)
		}
		if rows.Err() != nil {
			log.Fatal("report read failed")
		}
		if json.NewEncoder(os.Stdout).Encode(reports) != nil {
			log.Fatal("output failed")
		}
		return
	}
	var r report
	var image []byte
	var extras [][]byte
	err = pool.QueryRow(ctx, `SELECT f.id::text,f.resource,f.kind::text,f.body,f.status::text,f.created_at,CASE WHEN f.created_at>=now()-interval '30 days' THEN f.attachment ELSE NULL END,CASE WHEN f.created_at>=now()-interval '30 days' THEN f.extra_attachments ELSE ARRAY[]::bytea[] END FROM feedback f JOIN projects p ON p.id=f.project_id WHERE p.slug=$1 AND f.id=$2`, *project, *id).Scan(&r.ID, &r.Resource, &r.Kind, &r.Body, &r.Status, &r.CreatedAt, &image, &extras)
	if err != nil {
		log.Fatal("report not found in this project")
	}
	r.HasAttachment = len(image) > 0
	r.AttachmentCount = len(extras)
	if r.HasAttachment {
		r.AttachmentCount++
	}
	// Refuse to overwrite an existing export, including symlink directories.
	if os.Mkdir(*output, 0700) != nil {
		log.Fatal("output must be a new directory")
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Fatal("report encoding failed")
	}
	if os.WriteFile(filepath.Join(*output, "report.json"), data, 0600) != nil {
		log.Fatal("report export failed")
	}
	if len(image) > 0 && os.WriteFile(filepath.Join(*output, "image.jpg"), image, 0600) != nil {
		log.Fatal("image export failed")
	}
	for i, extra := range extras {
		if os.WriteFile(filepath.Join(*output, fmt.Sprintf("image-%d.jpg", i+2)), extra, 0600) != nil {
			log.Fatal("image export failed")
		}
	}
	fmt.Println("Exported report to", *output)
}
