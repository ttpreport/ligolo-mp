package forms

type FormVal[T any] struct {
	Last T
	Hint string
}

type FormSelectVal struct {
	ID    int
	Value string
}

func (fsv *FormSelectVal) String() string {
	return fsv.Value
}
