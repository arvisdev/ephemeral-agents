package main

import (
	"github.com/google/uuid"
)

func defaultUUID() string {
	return uuid.NewString()
}
