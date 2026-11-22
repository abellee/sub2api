package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

const (
	GroupProtocolAnthropic = "anthropic"
	GroupProtocolOpenAI    = "openai"
	GroupProtocolGoogle    = "google"
)

func ValidateRequiredProtocol(protocol string) error {
	switch protocol {
	case "", GroupProtocolAnthropic, GroupProtocolOpenAI, GroupProtocolGoogle:
		return nil
	default:
		return infraerrors.BadRequest("INVALID_REQUIRED_PROTOCOL", "required_protocol must be empty, anthropic, openai or google")
	}
}
