package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMirrorResultsMapsUserByEmail(t *testing.T) {
	source, s, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	replica, r, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer replica.Close()
	at := time.Date(2026, 10, 11, 1, 2, 3, 0, time.UTC)
	value, _ := json.Marshal(syncRequest{UserID: 24, Enabled: false, UpdatedAt: at})
	s.ExpectQuery("SELECT key,value FROM settings").WithArgs(resultPrefix).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).AddRow(resultPrefix+"24", string(value)))
	s.ExpectQuery("SELECT lower").WithArgs(int64(24)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("sync@example.com"))
	r.ExpectQuery("SELECT id").WithArgs("sync@example.com").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(81))
	mapped, _ := json.Marshal(syncRequest{UserID: 81, Enabled: false, UpdatedAt: at})
	r.ExpectExec("INSERT INTO settings").WithArgs(resultPrefix+"81", string(mapped), at).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := mirrorResults(context.Background(), source, replica); err != nil {
		t.Fatal(err)
	}
	if err := s.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := r.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRelayRequestsMapsUserAndPreservesRevision(t *testing.T) {
	source, s, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	replica, r, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer replica.Close()
	at := time.Date(2026, 10, 11, 1, 2, 3, 0, time.UTC)
	value, _ := json.Marshal(syncRequest{UserID: 81, OrderID: 999, Enabled: false, UpdatedAt: at})
	r.ExpectQuery("SELECT key,value FROM settings").WithArgs(requestPrefix).WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).AddRow(requestPrefix+"81", string(value)))
	r.ExpectQuery("SELECT lower").WithArgs(int64(81)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("sync@example.com"))
	s.ExpectQuery("SELECT id").WithArgs("sync@example.com").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(24))
	mapped, _ := json.Marshal(syncRequest{UserID: 24, Enabled: false, UpdatedAt: at})
	s.ExpectExec("INSERT INTO settings").WithArgs(requestPrefix+"24", string(mapped), at).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := relayRequests(context.Background(), replica, source); err != nil {
		t.Fatal(err)
	}
	if err := s.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := r.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMappedUserMissingDoesNotUseSourceID(t *testing.T) {
	source, s, _ := sqlmock.New()
	defer source.Close()
	replica, r, _ := sqlmock.New()
	defer replica.Close()
	s.ExpectQuery("SELECT lower").WithArgs(int64(24)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("sync@example.com"))
	r.ExpectQuery("SELECT id").WithArgs("sync@example.com").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	id, err := mappedUserID(context.Background(), source, replica, 24)
	if id != 0 || err != sql.ErrNoRows {
		t.Fatalf("unexpected mapping %d %v", id, err)
	}
	if err := s.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := r.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMappedUserRejectsAmbiguousEmail(t *testing.T) {
	source, s, _ := sqlmock.New()
	defer source.Close()
	replica, r, _ := sqlmock.New()
	defer replica.Close()
	s.ExpectQuery("SELECT lower").WithArgs(int64(24)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("sync@example.com"))
	r.ExpectQuery("SELECT id").WithArgs("sync@example.com").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(81).AddRow(82))
	id, err := mappedUserID(context.Background(), source, replica, 24)
	if id != 0 || err == nil {
		t.Fatalf("ambiguous mapping accepted: %d %v", id, err)
	}
	if err := s.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := r.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
