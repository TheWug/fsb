package api

import (
	"github.com/thewug/fsb/pkg/api/types"

	"reflect"
	"testing"
	"errors"
)

func Test_SanitizeRating(t *testing.T) {
	testcases := map[string]struct{
		inputRatingString string
		expectedRating types.PostRating
		expectedError error
	}{
		"rating-e": {"e", types.Explicit, nil},
		"rating-q": {"q", types.Questionable, nil},
		"rating-s": {"s", types.Safe, nil},
		"rating-c": {"c", types.Safe, nil},
		"rating-o": {"o", types.Invalid, errors.New("Invalid rating")},
		"rating-e-full": {"explicit", types.Explicit, nil},
		"rating-q-full": {"questionable", types.Questionable, nil},
		"rating-s-full": {"safe", types.Safe, nil},
		"rating-c-full": {"clean", types.Safe, nil},
		"rating-o-full": {"original", types.Invalid, errors.New("Invalid rating")},
		"rating-junk": {"garbage", types.Invalid, errors.New("Invalid rating")},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			out, err := SanitizeRating(v.inputRatingString)
			if out != v.expectedRating { t.Errorf("Unexpected result: got %v, expected %v", out, v.expectedRating) }
			if !reflect.DeepEqual(err, v.expectedError) { t.Errorf("Unexpected error: got %v, expected %v", out, v.expectedRating) }
		})
	}
}

func Test_SanitizeRatingForEdit(t *testing.T) {
	testcases := map[string]struct{
		inputRatingString string
		expectedRating types.PostRating
		expectedError error
	}{
		"rating-e": {"e", types.Explicit, nil},
		"rating-q": {"q", types.Questionable, nil},
		"rating-s": {"s", types.Safe, nil},
		"rating-c": {"c", types.Safe, nil},
		"rating-o": {"o", types.Original, nil},
		"rating-e-full": {"explicit", types.Explicit, nil},
		"rating-q-full": {"questionable", types.Questionable, nil},
		"rating-s-full": {"safe", types.Safe, nil},
		"rating-c-full": {"clean", types.Safe, nil},
		"rating-o-full": {"original", types.Original, nil},
		"rating-junk": {"garbage", types.Invalid, errors.New("Invalid rating")},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			out, err := SanitizeRatingForEdit(v.inputRatingString)
			if out != v.expectedRating { t.Errorf("Unexpected result: got %v, expected %v", out, v.expectedRating) }
			if !reflect.DeepEqual(err, v.expectedError) { t.Errorf("Unexpected error: got %v, expected %v", out, v.expectedRating) }
		})
	}
}
