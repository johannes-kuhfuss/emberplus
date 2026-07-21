package ember

// Glow 2.50 parameter types.
const (
	ParameterTypeNone    = 0
	ParameterTypeInteger = 1
	ParameterTypeReal    = 2
	ParameterTypeString  = 3
	ParameterTypeBoolean = 4
	ParameterTypeTrigger = 5
	ParameterTypeEnum    = 6
	ParameterTypeOctets  = 7
)

// Glow parameter access flags.
const (
	AccessNone      = 0
	AccessRead      = 1
	AccessWrite     = 2
	AccessReadWrite = 3
)

// GetDirectory field masks.
const (
	FieldSparse      int64 = -2
	FieldAll         int64 = -1
	FieldDefault     int64 = 0
	FieldIdentifier  int64 = 1
	FieldDescription int64 = 2
	FieldTree        int64 = 3
	FieldValue       int64 = 4
	FieldConnections int64 = 5
)

// Matrix types and addressing modes.
const (
	MatrixOneToN   = 0
	MatrixOneToOne = 1
	MatrixNToN     = 2

	MatrixAddressingLinear    = 0
	MatrixAddressingNonLinear = 1
)

// Matrix connection operations and dispositions.
const (
	ConnectionAbsolute   int64 = 0
	ConnectionConnect    int64 = 1
	ConnectionDisconnect int64 = 2

	DispositionTally    int64 = 0
	DispositionModified int64 = 1
	DispositionPending  int64 = 2
	DispositionLocked   int64 = 3
)

// Binary stream formats.
const (
	StreamUnsignedInt8              int64 = 0
	StreamUnsignedInt16BigEndian    int64 = 2
	StreamUnsignedInt16LittleEndian int64 = 3
	StreamUnsignedInt32BigEndian    int64 = 4
	StreamUnsignedInt32LittleEndian int64 = 5
	StreamUnsignedInt64BigEndian    int64 = 6
	StreamUnsignedInt64LittleEndian int64 = 7
	StreamSignedInt8                int64 = 8
	StreamSignedInt16BigEndian      int64 = 10
	StreamSignedInt16LittleEndian   int64 = 11
	StreamSignedInt32BigEndian      int64 = 12
	StreamSignedInt32LittleEndian   int64 = 13
	StreamSignedInt64BigEndian      int64 = 14
	StreamSignedInt64LittleEndian   int64 = 15
	StreamIEEEFloat32BigEndian      int64 = 20
	StreamIEEEFloat32LittleEndian   int64 = 21
	StreamIEEEFloat64BigEndian      int64 = 22
	StreamIEEEFloat64LittleEndian   int64 = 23
)
