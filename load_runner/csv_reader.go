package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

type Account struct {
	PrivateKey   string
	TokenAddress string
	Decimal      string
	Balance      string
	WalletTier   string
	WalletIndex  string
	SourceWallet string
}

func ReadAccountsFromCSV(filePath string) ([]Account, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file is empty or has no data rows")
	}

	var accounts []Account
	for i, record := range records[1:] {
		if len(record) < 7 {
			return nil, fmt.Errorf("invalid record at row %d: expected 7 columns, got %d", i+2, len(record))
		}

		privateKey := strings.TrimSpace(record[0])
		if !strings.HasPrefix(privateKey, "0x") {
			privateKey = "0x" + privateKey
		}

		account := Account{
			PrivateKey:   privateKey,
			TokenAddress: strings.TrimSpace(record[1]),
			Decimal:      strings.TrimSpace(record[2]),
			Balance:      strings.TrimSpace(record[3]),
			WalletTier:   strings.TrimSpace(record[4]),
			WalletIndex:  strings.TrimSpace(record[5]),
			SourceWallet: strings.TrimSpace(record[6]),
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}