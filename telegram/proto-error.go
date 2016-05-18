package telegram

import (
	"errors"
	"fmt"
)

func HandleSoftError(resp *TGenericResponse) (error) {
	if resp.Ok != true {
		return errors.New(fmt.Sprintf("Failure indicated by API endpoint (%d: %s)\n", *resp.Error_code, *resp.Description))
	}
	return nil
}
