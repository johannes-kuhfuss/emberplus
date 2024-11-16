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

func TestNewDecoder(t *testing.T) {
	t.Parallel()

	type args struct {
		b []byte
	}

	tests := []struct {
		name string
		args args
		want *Decoder
	}{
		{
			"+valid",
			args{
				[]byte{0x00, 0x01, 0x02},
			},
			&Decoder{bytes.NewBuffer([]byte{0x00, 0x01, 0x02})},
		},
		{
			"+empty",
			args{
				[]byte{},
			},
			&Decoder{bytes.NewBuffer([]byte{})},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewDecoder(tt.args.b)
			if diff := cmp.Diff(tt.want.data, got.data, cmp.AllowUnexported(bytes.Buffer{})); diff != "" {
				t.Fatalf("NewDecoder() = %s", diff)
			}
		})
	}
}

func TestDecoderBytes(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name   string
		fields fields
		want   []byte
	}{
		{
			"+valid",
			fields{
				bytes.NewBuffer([]byte{0x00, 0x01, 0x02}),
			},
			[]byte{0x00, 0x01, 0x02},
		},
		{
			"+empty",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			[]byte{},
		},
		{
			"-nil",
			fields{
				nil,
			},
			nil,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got := c.Bytes()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.Bytes() = %s", diff)
			}
		})
	}
}

func TestDecoderLen(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			"+valid",
			fields{bytes.NewBuffer([]byte{0x01, 0x02, 0x03, 0x04})},
			4,
		},
		{
			"+empty",
			fields{bytes.NewBuffer([]byte{})},
			0,
		},
		{
			"+nilData",
			fields{nil},
			0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Decoder{
				data: tt.fields.data,
			}
			got := c.Len()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Decoder.Len() = %s", diff)
			}
		})
	}
}

func TestNewEncoder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want *Encoder
	}{
		{
			"+valid",
			&Encoder{bytes.NewBuffer(nil)},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewEncoder()
			if diff := cmp.Diff(tt.want.data, got.data, cmp.AllowUnexported(bytes.Buffer{})); diff != "" {
				t.Fatalf("NewEncoder() = %s", diff)
			}
		})
	}
}
func TestDecodeAny(t *testing.T) {
	t.Parallel()

	var boolVar bool

	boolVarTrue := true

	var stringVar string

	stringVarSet := "Ruby"

	type args struct {
		in  []byte
		val any
	}

	tests := []struct {
		name      string
		args      args
		wantValue any
		wantLen   int
		wantErr   bool
	}{
		{
			"+bool",
			args{
				[]byte{0x01, 0x01, 0xff},
				&boolVar,
			},
			&boolVarTrue,
			3,
			false,
		},
		{
			"+string",
			args{
				[]byte{0x0C, 0x04, 0x52, 0x75, 0x62, 0x79},
				&stringVar,
			},
			&stringVarSet,
			6,
			false,
		},
		{
			"+stringAdditional",
			args{
				[]byte{0x0C, 0x04, 0x52, 0x75, 0x62, 0x79, 0x01, 0x01, 0xff},
				&stringVar,
			},
			&stringVarSet,
			6,
			false,
		},
		{
			"-notPointerInput",
			args{
				[]byte{0x0C, 0x04, 0x52, 0x75, 0x62, 0x79},
				stringVar,
			},
			"",
			0,
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := DecodeAny(tt.args.in, tt.args.val)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DecodeAny() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.wantValue, tt.args.val); diff != "" {
				t.Fatalf("DecodeAny() val = %s", diff)
			}

			if diff := cmp.Diff(tt.wantLen, got); diff != "" {
				t.Fatalf("DecodeAny() len = %s", diff)
			}
		})
	}
}

func TestApplicationByte(t *testing.T) {
	t.Parallel()

	type args struct {
		num uint8
	}

	tests := []struct {
		name string
		args args
		want uint8
	}{
		{"+valid", args{0x30}, 0x70},
		{"+zero", args{0x00}, 0x60},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ApplicationByte(tt.args.num)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("ApplicationByte() = %s", diff)
			}
		})
	}
}

func TestContextByte(t *testing.T) {
	t.Parallel()

	type args struct {
		num uint8
	}

	tests := []struct {
		name string
		args args
		want uint8
	}{
		{"+valid", args{0x30}, 0xb0},
		{"+zero", args{0x00}, 0xa0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ContextByte(tt.args.num)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("ContextByte() = %s", diff)
			}
		})
	}
}

func TestUniversalByte(t *testing.T) {
	t.Parallel()

	type args struct {
		num uint8
	}

	tests := []struct {
		name string
		args args
		want uint8
	}{
		{"+valid", args{0x30}, 0x30},
		{"+zero", args{0x00}, 0x00},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := UniversalByte(tt.args.num)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("UniversalByte() = %s", diff)
			}
		})
	}
}
