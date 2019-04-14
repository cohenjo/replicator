package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type namesRecord struct {
	Name   string `db:"name"`
	Year   int    `db:"year"`
	Gender string `db:"gender"`
	Count  int    `db:"count"`
}

func main() {

	var user string
	var pass string
	var host string
	var db string
	flag.StringVar(&user, "user", "", "db user")
	flag.StringVar(&pass, "pass", "", "db pass")
	flag.StringVar(&host, "host", "", "the db host")
	flag.StringVar(&db, "db", "test", "the db schema")

	flag.Parse()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	file, err := os.Open("/tmp/NationalNames.csv")
	if err != nil {
		logger.Error().Err(err).Msg("failed to open file")
		return
	}
	defer file.Close()

	r := csv.NewReader(bufio.NewReader(file))
	r.Comma = ','
	r.Comment = '#'

	for i := 0; i < 10000; i++ {
		record, err := r.Read()
		if err == io.EOF {
			logger.Info().Err(err).Msg("Reached file end")
			break
		} else if err != nil {
			logger.Error().Err(err).Msg("Bad record")
			break
		}
		year, _ := strconv.Atoi(record[2])
		Count, _ := strconv.Atoi(record[4])
		r := namesRecord{
			Name:   record[1],
			Year:   year,
			Gender: record[3],
			Count:  Count,
		}
		logger.Info().Str("name", r.Name).Int("year", r.Year).Str("gender", r.Gender).Int("Count", r.Count).Msg("record")
		insrtStmt := `Insert into names values(unhex(replace(uuid(),'-','')),'%s',%d,'%s',%d)`

		streamUri := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?interpolateParams=true",
			user, pass, host, 3306, db,
		)
		conn, err := sqlx.Open("mysql", streamUri)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while connecting to source MySQL db")
		}

		tx := conn.MustBegin()
		res, err := tx.Exec(fmt.Sprintf(insrtStmt, r.Name, r.Year, r.Gender, r.Count))
		if err != nil {
			logger.Error().Err(err).Msgf("Error inserting to MySQL. res: %v", res)
		}
		err = tx.Commit()
		if err != nil {
			logger.Error().Err(err).Msgf("Error inserting to MySQL. res: %v", res)
		}

		// Don't finish too fast ~5h
		time.Sleep(20 * time.Millisecond)

	}
	logger.Info().Msg("done")

}
