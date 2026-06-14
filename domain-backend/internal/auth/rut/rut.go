// Package rut — issue-02.7 RUT chileno normalize + validate módulo 11.
//
// Formato canónico: NNNNNNNN-X (sin puntos, con guión, K mayúscula).
// Acepta inputs con puntos, sin puntos, con o sin guión.
package rut

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var (
	// ErrEmpty input vacío.
	ErrEmpty = errors.New("rut: empty")
	// ErrInvalidFormat formato no parsable.
	ErrInvalidFormat = errors.New("rut: invalid format")
	// ErrInvalidCheckDigit DV (módulo 11) incorrecto.
	ErrInvalidCheckDigit = errors.New("rut: invalid check digit")
	// ErrOutOfRange número fuera de rango razonable (1..99M).
	ErrOutOfRange = errors.New("rut: number out of range")
)

// Normalize convierte input a formato canónico NNNNNNNN-X.
// Acepta: "12.345.678-9", "12345678-9", "123456789", "12345678k".
func Normalize(raw string) (string, error) {
	if raw == "" {
		return "", ErrEmpty
	}
	// strip: puntos, espacios, lower→upper K
	clean := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsDigit(r):
			return r
		case r == 'k' || r == 'K':
			return 'K'
		case r == '-' || r == '.' || unicode.IsSpace(r):
			return -1
		}
		return -1
	}, raw)

	if clean == "" {
		return "", ErrInvalidFormat
	}
	if len(clean) < 2 || len(clean) > 9 {
		return "", ErrInvalidFormat
	}

	body := clean[:len(clean)-1]
	dv := clean[len(clean)-1:]

	// body debe ser numérico
	bodyN, err := strconv.Atoi(body)
	if err != nil {
		return "", ErrInvalidFormat
	}
	if bodyN < 1 || bodyN > 99_999_999 {
		return "", ErrOutOfRange
	}

	// dv debe ser dígito 0-9 o K
	if dv != "K" {
		if _, err := strconv.Atoi(dv); err != nil {
			return "", ErrInvalidFormat
		}
	}

	return fmt.Sprintf("%d-%s", bodyN, dv), nil
}

// Validate normaliza + valida dígito verificador módulo 11.
// Retorna RUT canónico si válido; error si no.
func Validate(raw string) (string, error) {
	canonical, err := Normalize(raw)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(canonical, "-", 2)
	bodyN, _ := strconv.Atoi(parts[0])
	dv := parts[1]
	expected := CheckDigit(bodyN)
	if dv != expected {
		return "", ErrInvalidCheckDigit
	}
	return canonical, nil
}

// CheckDigit calcula DV (0-9 o K) de un número body usando módulo 11.
// Algoritmo Chile: pesos 2,3,4,5,6,7 cíclicos desde el dígito menos significativo.
func CheckDigit(body int) string {
	sum := 0
	weight := 2
	n := body
	for n > 0 {
		sum += (n % 10) * weight
		n /= 10
		weight++
		if weight > 7 {
			weight = 2
		}
	}
	rem := 11 - (sum % 11)
	switch rem {
	case 11:
		return "0"
	case 10:
		return "K"
	}
	return strconv.Itoa(rem)
}

// IsValid wrapper boolean.
func IsValid(raw string) bool {
	_, err := Validate(raw)
	return err == nil
}
