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

func TestEncoder_GetData(t *testing.T) {
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
			fields{bytes.NewBuffer([]byte{0x00, 0x01, 0x02})},
			[]byte{0x00, 0x01, 0x02},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			got := c.GetData()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Encoder.GetData() = %s", diff)
			}
		})
	}
}

func TestEncoder_WriteRequest(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		path []int
		tag  string
		cmd  int
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			"+validNode",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, NodeType, EmberGetDirCommand},
			[]byte{
				0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x6a, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x20, 0xa1, 0x03, 0x02, 0x01, 0xff, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+validQualifiedNode",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, QualifiedNodeType, EmberGetDirCommand},
			[]byte{
				0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x6a, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x20, 0xa1, 0x03, 0x02, 0x01, 0xff, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+validParameter",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, ParameterType, EmberGetDirCommand},
			[]byte{
				0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x69, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x20, 0xa1, 0x03, 0x02, 0x01, 0xff, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+validQualifiedParameter",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, QualifiedParameterType, EmberGetDirCommand},
			[]byte{
				0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x69, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x20, 0xa1, 0x03, 0x02, 0x01, 0xff, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+validFunction",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, FunctionType, EmberGetDirCommand},
			[]byte{
				0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x74, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x20, 0xa1, 0x03, 0x02, 0x01, 0xff, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+existingData",
			fields{
				bytes.NewBuffer([]byte{0x00}),
			},
			args{[]int{}, FunctionType, EmberGetDirCommand},
			[]byte{
				0x00, 0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x74, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x20, 0xa1, 0x03, 0x02, 0x01, 0xff, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+unsubscribe",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, ParameterType, EmberGetUnsubscribeCommand},
			[]byte{
				0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x69, 0x80, 0xa0, 0x80, 0x0d, 0x00, 0xa2, 0x80, 0x64, 0x80,
				0xa0, 0x80, 0x62, 0x80, 0xa0, 0x03, 0x02, 0x01, 0x1f, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"-invalidType",
			fields{
				bytes.NewBuffer([]byte{}),
			},
			args{[]int{}, "foobar", EmberGetDirCommand},
			[]byte{0x60, 0x80, 0x6b, 0x80, 0xa0, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			if err := c.WriteRequest(tt.args.path, tt.args.tag, tt.args.cmd); (err != nil) != tt.wantErr {
				t.Fatalf("Encoder.WriteRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.GetData() = %s", diff)
			}
		})
	}
}

func TestEncoder_WriteUniversal(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		path []int
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			"+valid",
			fields{bytes.NewBuffer(nil)},
			args{
				[]int{1},
			},
			[]byte{
				0x0D, 0x01, 0x01,
			},
		},
		{
			"+multiple",
			fields{bytes.NewBuffer(nil)},
			args{
				[]int{1, 2},
			},
			[]byte{
				0x0D, 0x02, 0x01, 0x02,
			},
		},
		{
			"+preExistingData",
			fields{bytes.NewBuffer([]byte{0x00, 0x00})},
			args{
				[]int{1, 2},
			},
			[]byte{
				0x00, 0x00, 0x0D, 0x02, 0x01, 0x02,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			c.WriteUniversal(tt.args.path)

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.WriteUniversal() = %s", diff)
			}
		})
	}
}

func TestEncoder_WriteRootTreeRequest(t *testing.T) {
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
			fields{bytes.NewBuffer([]byte{})},
			[]byte{
				0x60, 0x80, 0x6B, 0x80, 0xA0, 0x80, 0x62, 0x80, 0xA0, 0x03, 0x02, 0x01, 0x20, 0xA1, 0x03, 0x02, 0x01,
				0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+existingData",
			fields{bytes.NewBuffer([]byte{0x00, 0x00})},
			[]byte{
				0x00, 0x00, 0x60, 0x80, 0x6B, 0x80, 0xA0, 0x80, 0x62, 0x80, 0xA0, 0x03, 0x02, 0x01, 0x20, 0xA1, 0x03,
				0x02, 0x01, 0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			if err := c.WriteRootTreeRequest(); (err != nil) != tt.wantErr {
				t.Fatalf("Encoder.WriteRootTreeRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.WriteRootTreeRequest() = %s", diff)
			}
		})
	}
}

func TestEncoder_WriteGetDirCommand(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		cmd int
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
			fields{bytes.NewBuffer([]byte{})},
			args{EmberGetDirCommand},
			[]byte{
				0xa0, 0x80, 0x62, 0x80, 0xA0, 0x03, 0x02, 0x01, 0x20, 0xA1, 0x03, 0x02, 0x01,
				0xFF, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
		{
			"+existingData",
			fields{bytes.NewBuffer([]byte{0x00, 0x00})},
			args{EmberGetDirCommand},
			[]byte{
				0x00, 0x00, 0xa0, 0x80, 0x62, 0x80, 0xA0, 0x03, 0x02, 0x01, 0x20, 0xA1, 0x03, 0x02, 0x01,
				0xFF, 0x00, 0x00, 0x00, 0x00,
			},
			false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			if err := c.WriteCommand(tt.args.cmd); (err != nil) != tt.wantErr {
				t.Fatalf("Encoder.WriteGetDirCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.WriteGetDirCommand() = %s", diff)
			}
		})
	}
}

func TestEncoder_writeInt(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		i    int
		cont uint8
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
			fields{bytes.NewBuffer([]byte{})},
			args{
				1,
				0xa0,
			},
			[]byte{0xa0, 0x03, 0x02, 0x01, 0x01},
			false,
		},
		{
			"+existingData",
			fields{bytes.NewBuffer([]byte{0x00, 0x00})},
			args{
				1,
				0xa0,
			},
			[]byte{0x00, 0x00, 0xa0, 0x03, 0x02, 0x01, 0x01},
			false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			if err := c.writeInt(tt.args.i, tt.args.cont); (err != nil) != tt.wantErr {
				t.Fatalf("Encoder.writeInt() error = %v, wantErr %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.writeInt() = %s", diff)
			}
		})
	}
}

func TestEncoder_openSequence(t *testing.T) {
	t.Parallel()

	type fields struct {
		data *bytes.Buffer
	}

	type args struct {
		appl byte
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   []byte
	}{
		{
			"+valid",
			fields{bytes.NewBuffer([]byte{})},
			args{0xa4},
			[]byte{0xa4, 0x80},
		},
		{
			"+existingData",
			fields{bytes.NewBuffer([]byte{0x00})},
			args{0xa4},
			[]byte{0x00, 0xa4, 0x80},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			c.openSequence(tt.args.appl)

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.openSequence() = %s", diff)
			}
		})
	}
}

func TestEncoder_closeSequence(t *testing.T) {
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
			fields{bytes.NewBuffer([]byte{})},
			[]byte{0x00, 0x00},
		},
		{
			"+existingData",
			fields{bytes.NewBuffer([]byte{0x00})},
			[]byte{0x00, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := &Encoder{
				data: tt.fields.data,
			}
			c.closeSequence()

			if diff := cmp.Diff(tt.want, c.data.Bytes()); diff != "" {
				t.Fatalf("Encoder.closeSequence() = %s", diff)
			}
		})
	}
}
