package model


// Add the functionality to extract the headers with the same key from a HttpReqDatum
func (m *HttpReqDatum) FilterHeaders(key string) []*HttpHeader {
	if m != nil {
		res := make([]*HttpHeader, 0)
		for _,h := range m.Headers {
			if h.Key == key {
				res = append(res, h)
			}
		}
		return res
	}
	return nil
}

// Add the functionality to extract the headers with the same key from a HttpRespDatum
func (m *HttpRespDatum) FilterHeaders(key string) []*HttpHeader {
	if m != nil {
		res := make([]*HttpHeader, 0)
		for _,h := range m.Headers {
			if h.Key == key {
				res = append(res, h)
			}
		}
		return res
	}
	return nil
}
