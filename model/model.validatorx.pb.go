// Manually generated to add Oneof validation to Datum types.

package model

import "fmt"

func (this *Datum) ValidateX() error {
	switch x := this.Val.(type) {
	case *EmptyDatum:
	case *BlobDatum:
	case *ErrorDatum:
	case *StageRefDatum:
	case *HTTPReqDatum:
	case *HTTPRespDatum:
	case *StateDatum:
	case nil:
		// The field is not set.
		return fmt.Errorf("Datum.Val must be set")
	default:
		// Catch if this file needs updating.
		return fmt.Errorf("Datum.Val has unexpected type %T", x)
	}
	return nil
}