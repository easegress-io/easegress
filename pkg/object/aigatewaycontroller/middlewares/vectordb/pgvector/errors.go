package pgvector

type ErrCreatePostgresClient struct {
	Message string
	Err     error
}

// NewErrCreatePostgresClient creates a new ErrCreatePostgresClient with the given message and error.
func NewErrCreatePostgresClient(message string, err error) *ErrCreatePostgresClient {
	return &ErrCreatePostgresClient{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreatePostgresClient) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrEnableVectorExtension struct {
	Message string
	Err     error
}

// NewErrEnableVectorExtension creates a new ErrEnableVectorExtension with the given message and error.
func NewErrEnableVectorExtension(message string, err error) *ErrEnableVectorExtension {
	return &ErrEnableVectorExtension{
		Message: message,
		Err:     err,
	}
}

func (e *ErrEnableVectorExtension) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrBeginTransaction struct {
	Message string
	Err     error
}

// NewErrBeginTransaction creates a new ErrBeginTransaction with the given message and error.
func NewErrBeginTransaction(message string, err error) *ErrBeginTransaction {
	return &ErrBeginTransaction{
		Message: message,
		Err:     err,
	}
}

func (e *ErrBeginTransaction) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrUnexpectedSchemaType struct {
	Message string
	Err     error
}

// NewErrUnexpectedSchemaType creates a new ErrUnexpectedSchemaType with the given message and error.
func NewErrUnexpectedSchemaType(message string, err error) *ErrUnexpectedSchemaType {
	return &ErrUnexpectedSchemaType{
		Message: message,
		Err:     err,
	}
}

func (e *ErrUnexpectedSchemaType) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrCreatePostgresDB struct {
	Message string
	Err     error
}

// NewErrCreatePostgresDB creates a new ErrCreatePostgresDB with the given message and error.
func NewErrCreatePostgresDB(message string, err error) *ErrCreatePostgresDB {
	return &ErrCreatePostgresDB{
		Message: message,
		Err:     err,
	}
}

func (e *ErrCreatePostgresDB) Error() string {
	return e.Message + ": " + e.Err.Error()
}

type ErrInsertDocuments struct {
	Message string
	Err     error
}

// NewErrInsertDocuments creates a new ErrInsertDocuments with the given message and error.
func NewErrInsertDocuments(message string, err error) *ErrInsertDocuments {
	return &ErrInsertDocuments{
		Message: message,
		Err:     err,
	}
}

func (e *ErrInsertDocuments) Error() string {
	return e.Message + ": " + e.Err.Error()
}
