package types

import (
	"strconv"
)

type AgeStatus int
const AGE_LOCKED      AgeStatus = -1
const AGE_UNVALIDATED AgeStatus = 0
const AGE_VALIDATED   AgeStatus = 1
const AGE_VERIFIED    AgeStatus = 2
func (this AgeStatus) Display() string {
	return map[AgeStatus]string{AGE_LOCKED: "Unverified", AGE_UNVALIDATED: "Unverified", AGE_VALIDATED: "Verified", AGE_VERIFIED: "Verified \u2728"}[this]
}
func (this AgeStatus) String() string {
	return strconv.Itoa(int(this))
}

type RatingMode int
const FILTER_NONE         RatingMode = 2
const FILTER_EXPLICIT     RatingMode = 1
const FILTER_QUESTIONABLE RatingMode = 0
func (this RatingMode) Display() string {
	return map[RatingMode]string{FILTER_NONE: "Show all posts", FILTER_EXPLICIT: "Show safe and questionable posts", FILTER_QUESTIONABLE: "Show safe posts only"}[this]
}
func (this RatingMode) String() string {
	return strconv.Itoa(int(this))
}

type BlacklistMode int
const BLACKLIST_ON           BlacklistMode = 0
const BLACKLIST_OFF          BlacklistMode = 1
func (this BlacklistMode) Display() string {
	return map[BlacklistMode]string{BLACKLIST_ON: "Blacklist enabled", BLACKLIST_OFF: "Blacklist disabled"}[this]
}
func (this BlacklistMode) String() string {
	return strconv.Itoa(int(this))
}
