package micro

type AccessLogger func(b []byte, msg string)
type ServerLogger func(b []byte)
type OperationLogger func(b []byte)
