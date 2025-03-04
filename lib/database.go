package lib

import "database/sql"

func RowsToCard(rows *sql.Rows) ([]SavedCard, error) {
	var card []SavedCard

	for rows.Next() {
		var c SavedCard
		err := rows.Scan(&c.ID, &c.Email, &c.PaymentID, &c.Last_4)
		if err != nil {
			return nil, err
		}
		card = append(card, c)
	}
	return card, nil
}
