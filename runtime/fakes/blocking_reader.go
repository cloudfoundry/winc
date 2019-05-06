package fakes

type Reader struct {
	ReadCall struct {
		Receives struct {
			Bytes []byte
		}
		Returns struct {
			NumBytes int
			Error    error
		}
	}
}

func (r *Reader) Read(p []byte) (int, error) {
	r.ReadCall.Receives.Bytes = p

	for {
		//do stuff neverending
	}

	return r.ReadCall.Returns.NumBytes, r.ReadCall.Returns.Error
}

func (r *Reader) Close() error {
	return nil
}
