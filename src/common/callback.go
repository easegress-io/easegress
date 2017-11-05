package common

const (
	NORMAL_PRIORITY_CALLBACK   = "__NoRmAl_PrIoRiTy_CaLlBaCk"
	CRITICAL_PRIORITY_CALLBACK = "__CrItIcAl_PrIoRiTy_CaLlBaCk"
)

////

type NamedCallback struct {
	name     string
	callback interface{}
}

func NewNamedCallback(name string, callback interface{}) *NamedCallback {
	return &NamedCallback{
		name:     name,
		callback: callback,
	}
}

func (cb *NamedCallback) Name() string {
	return cb.name
}

func (cb *NamedCallback) Callback() interface{} {
	return cb.callback
}

func (cb *NamedCallback) SetCallback(callback interface{}) interface{} {
	oriCallback := cb.callback
	cb.callback = callback
	return oriCallback
}

////

func AddCallback(callbacks []*NamedCallback, name string, callback interface{},
	rewrite bool, priority string) (

	[]*NamedCallback, interface{}, bool) {

	var oriCallback interface{}
	for _, namedCallback := range callbacks {
		if namedCallback.Name() == name {
			if rewrite {
				oriCallback = namedCallback.SetCallback(callback)
			} else {
				return callbacks, namedCallback.Callback(), false
			}
		}
	}

	if oriCallback == nil {
		callback := NewNamedCallback(name, callback)
		if priority == NORMAL_PRIORITY_CALLBACK || callbacks == nil {
			callbacks = append(callbacks, callback)
		} else if priority == CRITICAL_PRIORITY_CALLBACK {
			callbacks = append([]*NamedCallback{callback}, callbacks...)
		} else {
			pos := len(callbacks)
			for i, namedCallback := range callbacks {
				if namedCallback.Name() == priority {
					pos = i
					break
				}
			}

			// insert before the pos
			callbacks = append(callbacks[:pos], append([]*NamedCallback{callback}, callbacks[pos:]...)...)
		}
	}

	return callbacks, oriCallback, true
}

func DeleteCallback(callbacks []*NamedCallback, name string) ([]*NamedCallback, interface{}) {
	var oriCallback interface{}
	for i, namedCallback := range callbacks {
		if namedCallback.Name() == name {
			oriCallback = namedCallback.Callback()
			callbacks = append(callbacks[:i], callbacks[i+1:]...)
			break
		}
	}

	return callbacks, oriCallback
}
