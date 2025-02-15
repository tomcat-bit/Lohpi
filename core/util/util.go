package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"bufio"
	"io"
	"net/http"
	"strings"
	log "github.com/sirupsen/logrus"
)

type MalformedParserReponse struct {
	Status int
	Msg    string
}

func (mr *MalformedParserReponse) Error() string {
	return mr.Msg
}

func DecodeJSONBody(w http.ResponseWriter, r *http.Request, contentType string, dst interface{}) error {
	if r.Header.Get("Content-Type") != contentType {
		msg := "Content-Type header is not " + contentType
		return &MalformedParserReponse{Status: http.StatusUnsupportedMediaType, Msg: msg}
	}

	// 1 MB max
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &MalformedParserReponse{Status: http.StatusBadRequest, Msg: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			return &MalformedParserReponse{Status: http.StatusBadRequest, Msg: msg}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &MalformedParserReponse{Status: http.StatusBadRequest, Msg: msg}

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
			return &MalformedParserReponse{Status: http.StatusBadRequest, Msg: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &MalformedParserReponse{Status: http.StatusBadRequest, Msg: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return &MalformedParserReponse{Status: http.StatusRequestEntityTooLarge, Msg: msg}

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &MalformedParserReponse{Status: http.StatusBadRequest, Msg: msg}
	}

	return nil
}

// Copies the headers from h into a new map of string slices.
func CopyHeaders(h map[string][]string) map[string][]string {
	m := make(map[string][]string)
	for key, val := range h {
		m[key] = val
	}
	return m
}

// Assigns the headers in dest from src
func SetHeaders(src, dest map[string][]string) {
	for k, v := range src {
		dest[k] = v
	}
}

// Streams the data from the response 'r' into the response writer 'w'.
func StreamToResponseWriter(r *bufio.Reader, w http.ResponseWriter, bufSize int) error {
	buffer := make([]byte, bufSize)

	for {
    	len, err := r.Read(buffer)
        if len > 0 {
			_, err = w.Write(buffer[:len])
			if err != nil {
				return err
			}
		}

        if err != nil {
            if err == io.EOF {
                log.Infoln(err.Error())
				return nil
            } else {
				return err
			}
        }
    }
	return nil 
}