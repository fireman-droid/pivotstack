package config

import (
	"fmt"
	"strings"
)

func ValidateGroupAliasUnique(exceptID, alias string) error {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return validateGroupAliasUniqueLocked(exceptID, alias)
}

func validateGroupAliasUniqueLocked(exceptID, alias string) error {
	if err := validateNewAPIAliasUniqueLocked(exceptID, alias); err != nil {
		return err
	}
	return validateDirectAliasUniqueLocked(exceptID, alias)
}

func validateNewAPIAliasUniqueLocked(exceptID, alias string) error {
	needle := normalizeGroupAlias(alias)
	if needle == "" || cfg == nil {
		return nil
	}
	exceptID = strings.TrimSpace(exceptID)
	for _, ch := range cfg.NewAPIChannels {
		if ch.DeletedAt > 0 || ch.ID == exceptID {
			continue
		}
		if normalizeGroupAlias(ch.Alias) == needle {
			return fmt.Errorf("alias %q conflicts with new-api channel %s", alias, ch.ID)
		}
	}
	return nil
}

func validateDirectAliasUniqueLocked(exceptID, alias string) error {
	needle := normalizeGroupAlias(alias)
	if needle == "" || cfg == nil {
		return nil
	}
	exceptID = strings.TrimSpace(exceptID)
	for _, ch := range cfg.DirectChannels {
		if ch.DeletedAt > 0 || ch.ID == exceptID {
			continue
		}
		if normalizeGroupAlias(ch.Alias) == needle {
			return fmt.Errorf("alias %q conflicts with direct channel %s", alias, ch.ID)
		}
	}
	return nil
}

func normalizeGroupAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}
