// Package jsonutil buendelt die beiden JSON-Regeln, an die sich der Port
// halten muss, um byteweise zum Node-Original zu passen: kein HTML-Escaping
// und keine Umsortierung von Schluesseln.
package jsonutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Marshal kodiert value als JSON, ohne <, > und & zu maskieren. Die
// Standardbibliothek maskiert sie, JSON.stringify im Browser nicht.
func Marshal(value any) ([]byte, error) {
	var buffer bytes.Buffer

	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}

	// Encode haengt immer einen Zeilenumbruch an.
	return bytes.TrimRight(buffer.Bytes(), "\n"), nil
}

// StripKey entfernt einen Schluessel der obersten Ebene aus einem JSON-Objekt
// und erhaelt dabei die Reihenfolge der uebrigen Schluessel. Der Umweg ueber
// map[string]any wuerde sie alphabetisch sortieren. Werte werden unveraendert
// uebernommen, lediglich Leerraum zwischen den Token faellt weg.
func StripKey(data []byte, key string) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, errors.New("JSON-Objekt erwartet")
	}

	var out bytes.Buffer
	out.WriteByte('{')

	first := true
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		name, ok := nameToken.(string)
		if !ok {
			return nil, fmt.Errorf("Zeichenkette als Schluessel erwartet, gefunden %v", nameToken)
		}

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		if name == key {
			continue
		}

		if !first {
			out.WriteByte(',')
		}
		first = false

		encodedName, err := Marshal(name)
		if err != nil {
			return nil, err
		}
		out.Write(encodedName)
		out.WriteByte(':')
		if err := json.Compact(&out, value); err != nil {
			return nil, err
		}
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unerwartete Daten nach dem JSON-Objekt")
	}

	out.WriteByte('}')
	return out.Bytes(), nil
}
