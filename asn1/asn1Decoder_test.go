/*
** Copyright (C) 2001-2024 Zabbix SIA
**
** This program is free software: you can redistribute it and/or modify it under the terms of
** the GNU Affero General Public License as published by the Free Software Foundation, version 3.
**
** This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;
** without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
** See the GNU Affero General Public License for more details.
**
** You should have received a copy of the GNU Affero General Public License along with this program.
** If not, see <https://www.gnu.org/licenses/>.
**/

package asn1

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDecoder_Read(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		tag         uint8
		compareByte func(num uint8) uint8
	}

	tests := []struct {
		name        string
		fields      fields
		args        args
		wantDecoder *Decoder
		wantLen     bool
		wantErr     bool
	}{
		{
			"+validApplication",
			fields{
				bytes.NewBuffer(
					[]byte{
						0x60, 0x34, 0x6B, 0x32, 0xA0, 0x30, 0x63, 0x2E, 0xA0, 0x03, 0x02, 0x01, 0x01, 0xA1, 0x27, 0x31,
						0x25, 0xA0, 0x16, 0x0C, 0x14, 0x52, 0x33, 0x4C, 0x41, 0x59, 0x56, 0x69, 0x72, 0x74, 0x75, 0x61,
						0x6C, 0x50, 0x61, 0x74, 0x63, 0x68, 0x42, 0x61, 0x79, 0xA1, 0x02, 0x0C, 0x00, 0xA4, 0x02, 0x0C,
						0x00, 0xA3, 0x03, 0x01, 0x01, 0xFF,
					},
				),
			},
			args{
				RootElementCollectionTag,
				ApplicationByte,
			},
			NewDecoder(
				[]byte{
					0x6B, 0x32, 0xA0, 0x30, 0x63, 0x2E, 0xA0, 0x03, 0x02, 0x01, 0x01, 0xA1, 0x27, 0x31, 0x25, 0xA0,
					0x16, 0x0C, 0x14, 0x52, 0x33, 0x4C, 0x41, 0x59, 0x56, 0x69, 0x72, 0x74, 0x75, 0x61, 0x6C, 0x50,
					0x61, 0x74, 0x63, 0x68, 0x42, 0x61, 0x79, 0xA1, 0x02, 0x0C, 0x00, 0xA4, 0x02, 0x0C, 0x00, 0xA3,
					0x03, 0x01, 0x01, 0xFF,
				},
			),
			false,
			false,
		},
		{
			"+validContext",
			fields{
				bytes.NewBuffer(
					[]byte{
						0xA0, 0x30, 0x63, 0x2E, 0xA0, 0x03, 0x02, 0x01, 0x01, 0xA1, 0x27, 0x31, 0x25, 0xA0, 0x16, 0x0C,
						0x14, 0x52, 0x33, 0x4C, 0x41, 0x59, 0x56, 0x69, 0x72, 0x74, 0x75, 0x61, 0x6C, 0x50, 0x61, 0x74,
						0x63, 0x68, 0x42, 0x61, 0x79, 0xA1, 0x02, 0x0C, 0x00, 0xA4, 0x02, 0x0C, 0x00, 0xA3, 0x03, 0x01,
						0x01, 0xFF,
					},
				),
			},
			args{
				0,
				ContextByte,
			},
			NewDecoder(
				[]byte{
					0x63, 0x2E, 0xA0, 0x03, 0x02, 0x01, 0x01, 0xA1, 0x27, 0x31, 0x25, 0xA0, 0x16, 0x0C, 0x14, 0x52,
					0x33, 0x4C, 0x41, 0x59, 0x56, 0x69, 0x72, 0x74, 0x75, 0x61, 0x6C, 0x50, 0x61, 0x74, 0x63, 0x68,
					0x42, 0x61, 0x79, 0xA1, 0x02, 0x0C, 0x00, 0xA4, 0x02, 0x0C, 0x00, 0xA3, 0x03, 0x01, 0x01, 0xFF,
				},
			),
			false,
			false,
		},
		{
			"+applicationWithThreeLenBytes",
			fields{
				bytes.NewBuffer(
					[]byte{
						0x60, 0x82, 0x01, 0x38, 0x6b, 0x82, 0x01, 0x34, 0xa0, 0x2d, 0x6a, 0x2b, 0xa0, 0x05, 0x0d, 0x03,
						0x01, 0x01, 0x05, 0xa1, 0x22, 0x31, 0x20, 0xa0, 0x11, 0x0c, 0x0f, 0x45, 0x6e, 0x76, 0x69, 0x72,
						0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x4c, 0x69, 0x73, 0x74, 0xa1, 0x02, 0x0c, 0x00, 0xa4, 0x02,
						0x0c, 0x00, 0xa3, 0x03, 0x01, 0x01, 0xff, 0xa0, 0x50, 0x69, 0x4e, 0xa0, 0x05, 0x0d, 0x03, 0x01,
						0x01, 0x01, 0xa1, 0x45, 0x31, 0x43, 0xa0, 0x13, 0x0c, 0x11, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f,
						0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x41, 0x63, 0x74, 0x69, 0x76, 0x65, 0xad, 0x03, 0x02, 0x01, 0x06,
						0xa5, 0x03, 0x02, 0x01, 0x03, 0xa7, 0x1d, 0x0c, 0x1b, 0x6e, 0x6f, 0x20, 0x65, 0x6e, 0x76, 0x69,
						0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x20, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x0a, 0x54,
						0x45, 0x53, 0x54, 0x0a, 0xa2, 0x03, 0x02, 0x01, 0x00, 0xa0, 0x38, 0x69, 0x36, 0xa0, 0x05, 0x0d,
						0x03, 0x01, 0x01, 0x02, 0xa1, 0x2d, 0x31, 0x2b, 0xa0, 0x10, 0x0c, 0x0e, 0x45, 0x6e, 0x76, 0x69,
						0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x43, 0x52, 0x43, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa5,
						0x03, 0x02, 0x01, 0x01, 0xa4, 0x03, 0x02, 0x01, 0xff, 0xa3, 0x03, 0x02, 0x01, 0x00, 0xa2, 0x03,
						0x02, 0x01, 0x00, 0xa0, 0x39, 0x69, 0x37, 0xa0, 0x05, 0x0d, 0x03, 0x01, 0x01, 0x03, 0xa1, 0x2e,
						0x31, 0x2c, 0xa0, 0x11, 0x0c, 0x0f, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e,
						0x74, 0x52, 0x6f, 0x77, 0x73, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa5, 0x03, 0x02, 0x01, 0x01, 0xa4,
						0x03, 0x02, 0x01, 0xff, 0xa3, 0x03, 0x02, 0x01, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x05, 0xa0, 0x3c,
						0x69, 0x3a, 0xa0, 0x05, 0x0d, 0x03, 0x01, 0x01, 0x04, 0xa1, 0x31, 0x31, 0x2f, 0xa0, 0x14, 0x0c,
						0x12, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6c, 0x75,
						0x6d, 0x6e, 0x73, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa5, 0x03, 0x02, 0x01, 0x01, 0xa4, 0x03, 0x02,
						0x01, 0xff, 0xa3, 0x03, 0x02, 0x01, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x03,
					},
				),
			},
			args{
				0,
				ApplicationByte,
			},
			NewDecoder(
				[]byte{
					0x6b, 0x82, 0x01, 0x34, 0xa0, 0x2d, 0x6a, 0x2b, 0xa0, 0x05, 0x0d, 0x03, 0x01, 0x01, 0x05, 0xa1,
					0x22, 0x31, 0x20, 0xa0, 0x11, 0x0c, 0x0f, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65,
					0x6e, 0x74, 0x4c, 0x69, 0x73, 0x74, 0xa1, 0x02, 0x0c, 0x00, 0xa4, 0x02, 0x0c, 0x00, 0xa3, 0x03,
					0x01, 0x01, 0xff, 0xa0, 0x50, 0x69, 0x4e, 0xa0, 0x05, 0x0d, 0x03, 0x01, 0x01, 0x01, 0xa1, 0x45,
					0x31, 0x43, 0xa0, 0x13, 0x0c, 0x11, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e,
					0x74, 0x41, 0x63, 0x74, 0x69, 0x76, 0x65, 0xad, 0x03, 0x02, 0x01, 0x06, 0xa5, 0x03, 0x02, 0x01,
					0x03, 0xa7, 0x1d, 0x0c, 0x1b, 0x6e, 0x6f, 0x20, 0x65, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d,
					0x65, 0x6e, 0x74, 0x20, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x0a, 0x54, 0x45, 0x53, 0x54, 0x0a,
					0xa2, 0x03, 0x02, 0x01, 0x00, 0xa0, 0x38, 0x69, 0x36, 0xa0, 0x05, 0x0d, 0x03, 0x01, 0x01, 0x02,
					0xa1, 0x2d, 0x31, 0x2b, 0xa0, 0x10, 0x0c, 0x0e, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d,
					0x65, 0x6e, 0x74, 0x43, 0x52, 0x43, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa5, 0x03, 0x02, 0x01, 0x01,
					0xa4, 0x03, 0x02, 0x01, 0xff, 0xa3, 0x03, 0x02, 0x01, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x00, 0xa0,
					0x39, 0x69, 0x37, 0xa0, 0x05, 0x0d, 0x03, 0x01, 0x01, 0x03, 0xa1, 0x2e, 0x31, 0x2c, 0xa0, 0x11,
					0x0c, 0x0f, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x6f, 0x77,
					0x73, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa5, 0x03, 0x02, 0x01, 0x01, 0xa4, 0x03, 0x02, 0x01, 0xff,
					0xa3, 0x03, 0x02, 0x01, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x05, 0xa0, 0x3c, 0x69, 0x3a, 0xa0, 0x05,
					0x0d, 0x03, 0x01, 0x01, 0x04, 0xa1, 0x31, 0x31, 0x2f, 0xa0, 0x14, 0x0c, 0x12, 0x45, 0x6e, 0x76,
					0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x43, 0x6f, 0x6c, 0x75, 0x6d, 0x6e, 0x73, 0xad,
					0x03, 0x02, 0x01, 0x01, 0xa5, 0x03, 0x02, 0x01, 0x01, 0xa4, 0x03, 0x02, 0x01, 0xff, 0xa3, 0x03,
					0x02, 0x01, 0x00, 0xa2, 0x03, 0x02, 0x01, 0x03,
				},
			),
			false,
			false,
		},
		{
			"+complex",
			fields{
				bytes.NewBuffer(
					[]byte{
						0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x6a, 0x80, 0xa0, 0x07, 0x0d, 0x05, 0x01, 0x01, 0x02, 0x04,
						0x03, 0xa2, 0x80, 0x64, 0x80, 0xa0, 0x80, 0x61, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x01, 0xa1, 0x80,
						0x31, 0x80, 0xa0, 0x04, 0x0c, 0x02, 0x4f, 0x6e, 0xad, 0x03, 0x02, 0x01, 0x04, 0xa2, 0x03, 0x01,
						0x01, 0x00, 0xa5, 0x03, 0x02, 0x01, 0x03, 0xa9, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x61, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x02, 0xa1, 0x80, 0x31,
						0x80, 0xa0, 0x0f, 0x0c, 0x0d, 0x43, 0x6f, 0x72, 0x72, 0x20, 0x47, 0x61, 0x69, 0x6e, 0x5b, 0x64,
						0x42, 0x5d, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa2, 0x03, 0x02, 0x01, 0x00, 0xa5, 0x03, 0x02, 0x01,
						0x03, 0xa9, 0x03, 0x01, 0x01, 0xff, 0xa4, 0x03, 0x02, 0x01, 0x00, 0xa3, 0x03, 0x02, 0x01, 0xf4,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01,
						0x03, 0xa1, 0x80, 0x31, 0x80, 0xa0, 0x0c, 0x0c, 0x0a, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73,
						0x73, 0x6f, 0x72, 0xa3, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x04, 0xa1, 0x80, 0x31, 0x80, 0xa0, 0x0a, 0x0c,
						0x08, 0x45, 0x78, 0x70, 0x61, 0x6e, 0x64, 0x65, 0x72, 0xa3, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x05, 0xa1,
						0x80, 0x31, 0x80, 0xa0, 0x06, 0x0c, 0x04, 0x47, 0x61, 0x74, 0x65, 0xa3, 0x03, 0x01, 0x01, 0xff,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01,
						0x06, 0xa1, 0x80, 0x31, 0x80, 0xa0, 0x09, 0x0c, 0x07, 0x44, 0x65, 0x45, 0x73, 0x73, 0x65, 0x72,
						0xa3, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					},
				),
			},
			args{
				RootElementCollectionTag,
				ApplicationByte,
			},
			NewDecoder(
				[]byte{
					0x6b, 0x80, 0xa0, 0x80, 0x6a, 0x80, 0xa0, 0x07, 0x0d, 0x05, 0x01, 0x01, 0x02, 0x04,
					0x03, 0xa2, 0x80, 0x64, 0x80, 0xa0, 0x80, 0x61, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x01, 0xa1, 0x80,
					0x31, 0x80, 0xa0, 0x04, 0x0c, 0x02, 0x4f, 0x6e, 0xad, 0x03, 0x02, 0x01, 0x04, 0xa2, 0x03, 0x01,
					0x01, 0x00, 0xa5, 0x03, 0x02, 0x01, 0x03, 0xa9, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x61, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x02, 0xa1, 0x80, 0x31,
					0x80, 0xa0, 0x0f, 0x0c, 0x0d, 0x43, 0x6f, 0x72, 0x72, 0x20, 0x47, 0x61, 0x69, 0x6e, 0x5b, 0x64,
					0x42, 0x5d, 0xad, 0x03, 0x02, 0x01, 0x01, 0xa2, 0x03, 0x02, 0x01, 0x00, 0xa5, 0x03, 0x02, 0x01,
					0x03, 0xa9, 0x03, 0x01, 0x01, 0xff, 0xa4, 0x03, 0x02, 0x01, 0x00, 0xa3, 0x03, 0x02, 0x01, 0xf4,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01,
					0x03, 0xa1, 0x80, 0x31, 0x80, 0xa0, 0x0c, 0x0c, 0x0a, 0x43, 0x6f, 0x6d, 0x70, 0x72, 0x65, 0x73,
					0x73, 0x6f, 0x72, 0xa3, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x04, 0xa1, 0x80, 0x31, 0x80, 0xa0, 0x0a, 0x0c,
					0x08, 0x45, 0x78, 0x70, 0x61, 0x6e, 0x64, 0x65, 0x72, 0xa3, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x05, 0xa1,
					0x80, 0x31, 0x80, 0xa0, 0x06, 0x0c, 0x04, 0x47, 0x61, 0x74, 0x65, 0xa3, 0x03, 0x01, 0x01, 0xff,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xa0, 0x80, 0x63, 0x80, 0xa0, 0x03, 0x02, 0x01,
					0x06, 0xa1, 0x80, 0x31, 0x80, 0xa0, 0x09, 0x0c, 0x07, 0x44, 0x65, 0x45, 0x73, 0x73, 0x65, 0x72,
					0xa3, 0x03, 0x01, 0x01, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				},
			),
			true,
			false,
		},
		{
			"+noDataAfterLenBytesNoLenDefined",
			fields{bytes.NewBuffer([]byte{0x60, 0x80})},
			args{RootElementCollectionTag, ApplicationByte},
			NewDecoder([]byte{}),
			true,
			false,
		},
		{
			"-incorrectCompareByte",
			fields{bytes.NewBuffer([]byte{0x80})},
			args{RootElementCollectionTag, ApplicationByte},
			nil,
			false,
			true,
		},
		{
			"-incorrectLenByte",
			fields{bytes.NewBuffer([]byte{0x60})},
			args{RootElementCollectionTag, ApplicationByte},
			nil,
			false,
			true,
		},
		{
			"-incorrectMultipleLenByte",
			fields{bytes.NewBuffer([]byte{0x60, 0x82, 0x01})},
			args{RootElementCollectionTag, ApplicationByte},
			nil,
			false,
			true,
		},
		{
			"-noDataAfterLenBytes",
			fields{bytes.NewBuffer([]byte{0x60, 0x82, 0x01, 0x38})},
			args{RootElementCollectionTag, ApplicationByte},
			nil,
			false,
			true,
		},
		{
			"-empty",
			fields{bytes.NewBuffer([]byte{})},
			args{0, ApplicationByte},
			nil,
			false,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, got1, err := c.Read(tt.args.tag, tt.args.compareByte)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.Read() error = %v, wantErr %v", err, tt.wantErr)
			}

			if (got == nil) && (tt.wantDecoder == nil) {
				return
			}

			if (got == nil) || (tt.wantDecoder == nil) {
				t.Fatalf("Decoder.Read() got = %v, want = %v", got, tt.wantDecoder)

				return
			}

			if diff := cmp.Diff(tt.wantDecoder.data.Bytes(), got.data.Bytes()); diff != "" {
				t.Fatalf("Decoder.Read() = %s", diff)
			}

			if got1 != tt.wantLen {
				t.Fatalf("Decoder.Read() got = %v, want = %v", got, tt.wantLen)

				return
			}
		})
	}
}

func TestDecoder_readLength(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name       string
		fields     fields
		want       int
		wantOffset int
		wantErr    bool
	}{
		{
			"+valid",
			fields{bytes.NewBuffer([]byte{0x34})},
			52,
			1,
			false,
		},
		{
			"+setByte",
			fields{bytes.NewBuffer([]byte{0x31, 0x80, 0xA0, 0x10, 0x0C})},
			49,
			1,
			false,
		},
		{
			"+multiLen",
			fields{bytes.NewBuffer([]byte{0x82, 0x01, 0x38})},
			312,
			3,
			false,
		},
		{
			"-failedToReadMulti",
			fields{bytes.NewBuffer([]byte{0x82, 0x01})},
			0,
			0,
			true,
		},
		{
			"-canNotReadLen",
			fields{bytes.NewBuffer([]byte{0x80, 0x01})},
			0,
			1,
			true,
		},
		{
			"-overMaxLen",
			fields{bytes.NewBuffer([]byte{0xff})},
			0,
			0,
			true,
		},
		{
			"-empty",
			fields{bytes.NewBuffer([]byte{})},
			0,
			0,
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, gotOffset, err := c.readLength()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.readLength() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.readLength() got = %s", diff)
			}

			if diff := cmp.Diff(tt.wantOffset, gotOffset); diff != "" {
				t.Fatalf("Decoder.readLength() got1 = %s", diff)
			}
		})
	}
}

func TestDecoder_ReadEnd(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name      string
		fields    fields
		want      bool
		wantBytes []byte
		wantErr   bool
	}{
		{
			"+notEnd",
			fields{
				data: bytes.NewBuffer(
					[]byte{0xa1, 0x02, 0x0c, 0x00, 0xa4, 0x02, 0x0c, 0x00, 0xa3, 0x03, 0x01, 0x01, 0xff},
				),
			},
			false,
			[]byte{0xa1, 0x02, 0x0c, 0x00, 0xa4, 0x02, 0x0c, 0x00, 0xa3, 0x03, 0x01, 0x01, 0xff},
			false,
		},
		{
			"+isEnd",
			fields{
				data: bytes.NewBuffer(
					[]byte{0x00, 0x00, 0x0c, 0x00, 0xa4, 0x02, 0x0c, 0x00, 0xa3, 0x03, 0x01, 0x01, 0xff},
				),
			},
			true,
			[]byte{0x0c, 0x00, 0xa4, 0x02, 0x0c, 0x00, 0xa3, 0x03, 0x01, 0x01, 0xff},
			false,
		},
		{
			"+isOnlyEnd",
			fields{
				data: bytes.NewBuffer(
					[]byte{0x00, 0x00},
				),
			},
			true,
			[]byte{},
			false,
		},
		{
			"+empty",
			fields{
				data: bytes.NewBuffer(nil),
			},
			true,
			nil,
			false,
		},
		{
			"-notEnoughLen",
			fields{
				data: bytes.NewBuffer(
					[]byte{0x00},
				),
			},
			false,
			[]byte{0x00},
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.ReadEnd()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.ReadEnd() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.wantBytes, c.Bytes()); diff != "" {
				t.Fatalf("Decoder.ReadEnd() = %s", diff)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.ReadEnd() = %s", diff)
			}
		})
	}
}

func TestDecoder_Peek(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name    string
		fields  fields
		want    byte
		wantErr bool
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer(
					[]byte{0x01, 0x02, 0x03},
				),
			},
			0x01,
			false,
		},
		{
			"-empty",
			fields{
				bytes.NewBuffer(
					[]byte{},
				),
			},
			0x00,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.Peek()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.Peek() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.Peek() = %s", diff)
			}
		})
	}
}

func TestDecoder_DecodeUniversal(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name    string
		fields  fields
		want    []int
		wantErr bool
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer(
					[]byte{
						UniversalObjectTag, 0x02, 0x01, 0x00,
					},
				),
			},
			[]int{1, 0},
			false,
		},
		{
			"-emptyBuffer",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			nil,
			true,
		},
		{
			"-invalidByteCount",
			fields{
				bytes.NewBuffer([]byte{UniversalObjectTag}),
			},
			nil,
			true,
		},
		{
			"-invalidLenByteCount",
			fields{
				bytes.NewBuffer([]byte{UniversalObjectTag, 0x02}),
			},
			nil,
			true,
		},
		{
			"-incorrectTag",
			fields{
				bytes.NewBuffer([]byte{0x01, 0x02, 0x01, 0x00}),
			},
			nil,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.DecodeUniversal()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.DecodeUniversal() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.DecodeUniversal() = %s", diff)
			}
		})
	}
}

func TestDecoder_DecodeUtf8(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name         string
		fields       fields
		want         string
		wantLeftover []byte
		wantErr      bool
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer(
					[]byte{
						0x0c, 0x82, 0x00, 0x89, 0x50, 0x54, 0x50, 0x20, 0x2d, 0x20, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d,
						0x20, 0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x0a, 0x50, 0x54, 0x50, 0x20, 0x2d,
						0x20, 0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x4d, 0x41, 0x44,
						0x49, 0x0a, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50, 0x20, 0x2d, 0x20, 0x57,
						0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x0a, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d, 0x20,
						0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50, 0x0a,
						0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50, 0x20,
						0x2d, 0x20, 0x4d, 0x41, 0x44, 0x49, 0x0a, 0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b,
						0x20, 0x2d, 0x20, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50,
					},
				),
			},
			//nolint:dupword
			"PTP - MADI - WordClock\nPTP - WordClock - MADI\nMADI - PTP - WordClock\nMADI - WordClock - PTP\n" +
				"WordClock - PTP - MADI\nWordClock - MADI - PTP",
			[]byte{},
			false,
		},
		{
			"+withLeftover",
			fields{
				bytes.NewBuffer(
					[]byte{
						0x0c, 0x82, 0x00, 0x89, 0x50, 0x54, 0x50, 0x20, 0x2d, 0x20, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d,
						0x20, 0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x0a, 0x50, 0x54, 0x50, 0x20, 0x2d,
						0x20, 0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x4d, 0x41, 0x44,
						0x49, 0x0a, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50, 0x20, 0x2d, 0x20, 0x57,
						0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x0a, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d, 0x20,
						0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50, 0x0a,
						0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50, 0x20,
						0x2d, 0x20, 0x4d, 0x41, 0x44, 0x49, 0x0a, 0x57, 0x6f, 0x72, 0x64, 0x43, 0x6c, 0x6f, 0x63, 0x6b,
						0x20, 0x2d, 0x20, 0x4d, 0x41, 0x44, 0x49, 0x20, 0x2d, 0x20, 0x50, 0x54, 0x50,

						0xA2, 0x03, 0x02, 0x01, 0x01, 0xA5, 0x03, 0x02, 0x01,
						0x01, 0xA9, 0x03, 0x01, 0x01, 0xFF, 0xA4, 0x03, 0x02, 0x01, 0x05, 0xA3, 0x03, 0x02, 0x01, 0x00,
						0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xA0, 0x80, 0x63, 0x80, 0xA0, 0x03, 0x02, 0x01,
						0x02, 0xA1, 0x80, 0x31, 0x80, 0xA0, 0x05, 0x0C, 0x03, 0x50, 0x54, 0x50, 0xA3, 0x03, 0x01, 0x01,
						0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xA0, 0x80, 0x63, 0x80, 0xA0, 0x03, 0x02,
						0x01, 0x03, 0xA1, 0x80, 0x31, 0x80, 0xA0, 0x06, 0x0C, 0x04, 0x4D, 0x41, 0x44, 0x49,
					},
				),
			},
			//nolint:dupword
			"PTP - MADI - WordClock\nPTP - WordClock - MADI\nMADI - PTP - WordClock\nMADI - WordClock - PTP\n" +
				"WordClock - PTP - MADI\nWordClock - MADI - PTP",
			[]byte{
				0xA2, 0x03, 0x02, 0x01, 0x01, 0xA5, 0x03, 0x02, 0x01, 0x01, 0xA9, 0x03, 0x01, 0x01, 0xFF, 0xA4, 0x03,
				0x02, 0x01, 0x05, 0xA3, 0x03, 0x02, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xA0,
				0x80, 0x63, 0x80, 0xA0, 0x03, 0x02, 0x01, 0x02, 0xA1, 0x80, 0x31, 0x80, 0xA0, 0x05, 0x0C, 0x03, 0x50,
				0x54, 0x50, 0xA3, 0x03, 0x01, 0x01, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xA0, 0x80,
				0x63, 0x80, 0xA0, 0x03, 0x02, 0x01, 0x03, 0xA1, 0x80, 0x31, 0x80, 0xA0, 0x06, 0x0C, 0x04, 0x4D, 0x41,
				0x44, 0x49,
			},
			false,
		},
		{
			"-emptyBuffer",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			"",
			[]byte{},
			true,
		},
		{
			"-invalidByteCount",
			fields{
				bytes.NewBuffer([]byte{UTF8StringTag}),
			},
			"",
			[]byte{},

			true,
		},
		{
			"-invalidLenByteCount",
			fields{
				bytes.NewBuffer([]byte{UTF8StringTag, 0x02}),
			},
			"",
			[]byte{},
			true,
		},
		{
			"-incorrectTag",
			fields{
				bytes.NewBuffer([]byte{0x01, 0x02, 0x01, 0x00}),
			},
			"",
			[]byte{0x02, 0x01, 0x00},
			true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.DecodeUTF8()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.DecodeUtf8() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.DecodeUtf8() = %s", diff)
			}

			if diff := cmp.Diff(tt.wantLeftover, c.data.Bytes()); diff != "" {
				t.Fatalf("Decoder.DecodeUtf8() leftover = %s", diff)
			}
		})
	}
}

func TestDecoder_DecodeInteger(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name    string
		fields  fields
		want    int
		wantErr bool
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer([]byte{0x02, 0x01, 0x01}),
			},
			1,
			false,
		},
		{
			"+twoByteInt",
			fields{
				bytes.NewBuffer([]byte{0x02, 0x02, 0x00, 0xFF}),
			},
			255,
			false,
		},
		{
			"-noLength",
			fields{
				bytes.NewBuffer([]byte{0x02}),
			},
			0,
			true,
		},
		{
			"-noAdditionalLength",
			fields{
				bytes.NewBuffer([]byte{0x02, 0x02}),
			},
			0,
			true,
		},
		{
			"-notIntTag",
			fields{
				bytes.NewBuffer([]byte{0x00}),
			},
			0,
			true,
		},
		{
			"-empty",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			0,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.DecodeInteger()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.DecodeInteger() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.DecodeInteger() = %s", diff)
			}
		})
	}
}

func TestDecoder_ReadByte(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name    string
		fields  fields
		want    byte
		wantErr bool
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer([]byte{0x01}),
			},
			0x01,
			false,
		},
		{
			"+multiple",
			fields{
				bytes.NewBuffer([]byte{0x01, 0x02, 0x03, 0x04}),
			},
			0x01,
			false,
		},
		{
			"-empty",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			0,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.ReadByte()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.ReadByte() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.ReadByte() = %s", diff)
			}
		})
	}
}

func TestDecoder_readWithOutLength(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer([]byte{1, 2, 3}),
			},
			[]byte{1, 2, 3},
			false,
		},
		{
			"+empty",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			[]byte{},
			false,
		},
		// as far as I know there is no way for io.ReadAll to return an error, it invokes Bytes buffer read, witch only
		// returns EOF and ReadAll does not return error in case of EOF.
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.readWithOutLength()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.readWithOutLength() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.readWithOutLength() = %s", diff)
			}
		})
	}
}

func TestDecoder_readWithLength(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		length int
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			"+valid",
			fields{bytes.NewBuffer([]byte{1, 2, 3})},
			args{3},
			[]byte{1, 2, 3},
			false,
		},
		{
			"-missMatchLength",
			fields{bytes.NewBuffer([]byte{1, 2, 3, 4})},
			args{5},
			nil,
			true,
		},
		{
			"-empty",
			fields{bytes.NewBuffer(nil)},
			args{5},
			nil,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got, err := c.readWithLength(tt.args.length)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Decoder.readWithLength() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.readWithLength() = %s", diff)
			}
		})
	}
}
