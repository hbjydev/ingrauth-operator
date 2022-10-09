package util

import "math/rand"

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var lowerRunes = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func RandLowerRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = lowerRunes[rand.Intn(len(lowerRunes))]
	}
	return string(b)
}
