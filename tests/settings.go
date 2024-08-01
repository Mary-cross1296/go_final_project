package tests

import (
	"log"
	"path/filepath"
)

var Port = 7540
var DBFile = getDBFilePath()
var FullNextDate = true
var Search = true
var Token = `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJwYXNzd29yZF9oYXNoIjoiJDJhJDEwJGRsdllwWWdwYmxvT2VTVEpCa0Q3WGViaktGc0NVN3RqNk9CTVEuVi9DMDdRMmVpOWIxWno2IiwiZXhwIjoxNzIyNDk4Nzc1fQ.EoSRi18aMO9Ly5-tETDguTkMw5m2L2Yy22jhAJ1Pto0`

func getDBFilePath() string {
	dbFile := "../backend/scheduler.db"
	absPath, err := filepath.Abs(dbFile)
	if err != nil {
		log.Fatalf("Error getting absolute path: %v", err)
	}
	log.Printf("Using database file: %s", absPath)
	return absPath
}
