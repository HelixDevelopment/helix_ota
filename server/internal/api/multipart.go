package api

import "mime/multipart"

// multipartForm is an alias for the standard parsed multipart form so the
// upload handler's part-resolution helpers read cleanly.
type multipartForm = multipart.Form

// readFilePart reads the first file part with the given field name fully into
// memory. It returns ok=false when the part is absent or cannot be opened.
func readFilePart(form *multipartForm, name string) ([]byte, bool) {
	if form == nil {
		return nil, false
	}
	files := form.File[name]
	if len(files) == 0 {
		return nil, false
	}
	f, err := files[0].Open()
	if err != nil {
		return nil, false
	}
	defer f.Close()
	b, err := readAll(f)
	if err != nil {
		return nil, false
	}
	return b, true
}

// readValuePart returns the first value of a non-file form field, ok=false when
// absent.
func readValuePart(form *multipartForm, name string) (string, bool) {
	if form == nil {
		return "", false
	}
	vals := form.Value[name]
	if len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}
