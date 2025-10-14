package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"

	"github.com/example/Yamato-Go-Gin-API/internal/tooling/db"
	"github.com/example/Yamato-Go-Gin-API/seeds"
)

func main() {
	//1.- Build the database connection string from environment variables.
	dsn, err := db.BuildPostgresDSNFromEnv()
	if err != nil {
		log.Fatalf("failed to build postgres dsn: %v", err)
	}

	//2.- Open a database connection using the pq driver.
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		//3.- Close the connection on exit to release resources.
		_ = db.Close()
	}()

	//4.- Verify the database is reachable before running the seeds.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	//5.- Construct the seeder instance that knows how to populate baseline data.
	seeder, err := seeds.NewSeeder(db)
	if err != nil {
		log.Fatalf("failed to initialize seeder: %v", err)
	}

	//6.- Execute the seeding workflow and surface any error to the operator.
	if err := seeder.Run(ctx); err != nil {
		log.Fatalf("seeding failed: %v", err)
	}

	//7.- Inform the operator that the bootstrap process finished without issues.
	log.Println("Database seed completed successfully")
}
