package nrw

type BaseErr struct {
	S string
}

func (e RetrieveErr) Error() string {
	return e.S
}

func (e RetrieveErr) String() string {
	return e.S
}

func (e RetrieveErr) Nil() bool {
	return len(e.S) == 0
}

type RetrieveErr struct {
	BaseErr
}

func NewRetrieveErr(s string) RetrieveErr {
	return RetrieveErr{BaseErr: BaseErr{S: s}}
}

type OperationErr struct {
	BaseErr
}

func NewOperaionErr(s string) OperationErr {
	return OperationErr{BaseErr: BaseErr{S: s}}
}

type StatErr struct {
	BaseErr
}

func NewStatErr(s string) StatErr {
	return StatErr{BaseErr: BaseErr{S: s}}
}
