package db

import (
	"log"
	"time"
)

func WriteToDB(ip, expression, result string) {
	const maxRetries = 5
	const retryDelay = 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := writeToDBOnce(ip, expression, result)
		if err != nil {
			if IsLockedError(err) {
				time.Sleep(retryDelay)
				continue
			}
			log.Println("Error writing to DB:", err)
			return
		}
		return
	}

	log.Println("Max retries reached for writing to DB")
}

func writeToDBOnce(ip, expression, result string) error {
	_, err := DB.Exec("INSERT INTO history (ip, expression, result) VALUES (?, ?, ?)", ip, expression, result)
	if err != nil {
		return err
	}
	return nil
}
