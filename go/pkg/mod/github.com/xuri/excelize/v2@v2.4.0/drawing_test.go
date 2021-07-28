// Copyright 2016 - 2021 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to
// and read from XLSX / XLSM / XLTM files. Supports reading and writing
// spreadsheet documents generated by Microsoft Excel™ 2007 and later. Supports
// complex components by high compatibility, and provided streaming API for
// generating or reading data from a worksheet with huge amounts of data. This
// library needs Go version 1.15 or later.

package excelize

import (
	"testing"
)

func TestDrawingParser(t *testing.T) {
	f := File{
		Drawings: make(map[string]*xlsxWsDr),
		XLSX: map[string][]byte{
			"charset": MacintoshCyrillicCharset,
			"wsDr":    []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"><xdr:oneCellAnchor><xdr:graphicFrame/></xdr:oneCellAnchor></xdr:wsDr>`)},
	}
	// Test with one cell anchor
	f.drawingParser("wsDr")
	// Test with unsupported charset
	f.drawingParser("charset")
}