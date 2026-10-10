package neweducate

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ggmolly/belfast/internal/protobuf"
)

const educateConditionCategory = "ShareCfg/child2_condition.json"

type educateConditionConfig struct {
	ID    uint32            `json:"id"`
	Type  uint32            `json:"type"`
	Param []json.RawMessage `json:"param"`
}

func decodeEducateConditionParam(params []json.RawMessage, index int, target any) error {
	if index >= len(params) {
		return fmt.Errorf("missing param[%d]", index)
	}
	raw := bytes.TrimSpace(params[index])
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return fmt.Errorf("param[%d] requires an explicit value", index)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("param[%d]: %w", index, err)
	}
	return nil
}

func educateCompare(a int64, op string, b int64) (bool, error) {
	switch op {
	case "=", "==":
		return a == b, nil
	case "!=", "~=":
		return a != b, nil
	case ">":
		return a > b, nil
	case ">=":
		return a >= b, nil
	case "<":
		return a < b, nil
	case "<=":
		return a <= b, nil
	}
	return false, fmt.Errorf("unknown comparison %q", op)
}
func educateKVCount(values []*protobuf.KVDATA, id uint32) int64 {
	for _, v := range values {
		if v.GetKey() == id {
			return int64(v.GetValue())
		}
	}
	return 0
}

// Evaluate every operand before reducing. An unsupported branch must not be
// concealed by short-circuit evaluation of another branch.
func evaluateEducateCondition(state *educateState, raw json.RawMessage) (bool, error) {
	return evaluateEducateConditionWithContext(state, raw, nil)
}
func evaluateEducateConditionDepth(state *educateState, raw json.RawMessage, depth int, context *educateConditionContext) (bool, error) {
	if depth > 64 {
		return false, fmt.Errorf("condition nesting exceeds 64")
	}
	var symbol string
	if json.Unmarshal(raw, &symbol) == nil {
		if symbol != "${num}" || context == nil || !context.hasNumber || context.Number < 0 || context.Number > int64(^uint32(0)) {
			return false, fmt.Errorf("invalid condition expression %s: numeric binding required", raw)
		}
		context.Multiplier = uint32(context.Number)
		context.usesNumber = true
		return context.Number > 0, nil
	}
	var id uint32
	if json.Unmarshal(raw, &id) == nil {
		config, ok, err := loadNewEducateConfigByID[educateConditionConfig](educateConditionCategory, id)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("%s/%d: missing condition", educateConditionCategory, id)
		}
		result, err := evaluateEducateConditionConfig(state, config, context)
		if err != nil {
			return false, fmt.Errorf("%s/%d/param: %w", educateConditionCategory, id, err)
		}
		return result, nil
	}
	var expression []json.RawMessage
	if err := json.Unmarshal(raw, &expression); err != nil || expression == nil {
		return false, fmt.Errorf("invalid condition expression %s", raw)
	}
	if len(expression) == 0 {
		return true, nil
	}
	var op string
	if len(expression) != 2 || json.Unmarshal(expression[0], &op) != nil || (op != "&&" && op != "||") {
		return false, fmt.Errorf("invalid logical condition %s", raw)
	}
	var terms []json.RawMessage
	if err := json.Unmarshal(expression[1], &terms); err != nil || len(terms) == 0 {
		return false, fmt.Errorf("logical condition has no operands")
	}
	result := op == "&&"
	var initial educateConditionContext
	if context != nil {
		initial = *context
	}
	var selected *educateConditionContext
	for _, term := range terms {
		operandContext := context
		if op == "||" && context != nil {
			copy := initial
			operandContext = &copy
		}
		value, err := evaluateEducateConditionDepth(state, term, depth+1, operandContext)
		if err != nil {
			return false, err
		}
		if op == "&&" {
			result = result && value
		} else {
			if value && context != nil {
				if selected != nil && (selected.usesNumber || operandContext.usesNumber) && (selected.Multiplier != operandContext.Multiplier || selected.hasNumber != operandContext.hasNumber || (selected.hasNumber && selected.Number != operandContext.Number)) {
					return false, fmt.Errorf("ambiguous numeric bindings in OR condition")
				}
				selected = operandContext
			}
			result = result || value
		}
	}
	if selected != nil {
		context.Number, context.Multiplier, context.hasNumber, context.usesNumber = selected.Number, selected.Multiplier, selected.hasNumber, selected.usesNumber
		context.window = selected.window
	}
	return result, nil
}
func evaluateEducateConditionConfig(state *educateState, c *educateConditionConfig, context *educateConditionContext) (bool, error) {
	p := c.Param
	decode := func(index int, target any) error {
		return decodeEducateConditionParam(p, index, target)
	}
	var value int64
	var op string
	var threshold int64
	switch c.Type {
	case 1:
		if len(p) != 4 {
			return false, fmt.Errorf("DROP requires four parameters")
		}
		var kind, id uint32
		if err := decode(0, &kind); err != nil {
			return false, err
		}
		if err := decode(1, &id); err != nil {
			return false, err
		}
		switch kind {
		case 1:
			if err := validateEducateConditionNumericOwner(state, newEducateAttrCategory, id); err != nil {
				return false, err
			}
			value = educateKVCount(state.Info.Res.Attrs, id)
		case 2:
			if err := validateEducateConditionNumericOwner(state, newEducateResourceCategory, id); err != nil {
				return false, err
			}
			value = educateKVCount(state.Info.Res.Resource, id)
		case 4:
			for _, buff := range state.Info.Benefit.GetActives() {
				if buff.GetId() == id {
					value = 1
				}
			}
		default:
			return false, fmt.Errorf("unsupported owned drop type %d", kind)
		}
		if err := decode(2, &op); err != nil {
			return false, err
		}
		if err := decode(3, &threshold); err != nil {
			return false, err
		}
	case 2, 4:
		if len(p) != 2 {
			return false, fmt.Errorf("type %d requires two parameters", c.Type)
		}
		if c.Type == 4 {
			value = int64(state.Info.Round.GetRound())
		} else {
			attrs, err := listNewEducateConfigs[newEducateAttrConfig](newEducateAttrCategory)
			if err != nil {
				return false, err
			}
			for _, attr := range attrs {
				if attr.Character == state.Info.GetId() && attr.Type == 1 {
					value += educateKVCount(state.Info.Res.Attrs, attr.ID)
				}
			}
		}
		if err := decode(0, &op); err != nil {
			return false, err
		}
		if err := decode(1, &threshold); err != nil {
			return false, err
		}
	case 3, 5:
		if len(p) != 3 {
			return false, fmt.Errorf("counter requires three parameters")
		}
		if c.Type == 3 {
			var id uint32
			if err := decode(0, &id); err != nil {
				return false, err
			}
			value = educateKVCount(state.Info.Site.EventCounter, id)
		} else {
			var ids []uint32
			if err := decode(0, &ids); err != nil {
				return false, err
			}
			for _, id := range ids {
				value += educateKVCount(state.Info.Site.WorkCounter, id)
			}
		}
		if err := decode(1, &op); err != nil {
			return false, err
		}
		if err := decode(2, &threshold); err != nil {
			return false, err
		}
	default:
		return evaluateEducateContextCondition(state, c, context)
	}
	bindEducateConditionNumber(context, value)
	return educateCompare(value, op, threshold)
}

// Compatibility saves may retain fields from another character. The client
// receives only the current character's numeric fields; gates must enforce the
// same ownership contract before consulting those retained values.
func validateEducateConditionNumericOwner(state *educateState, category string, id uint32) error {
	config, found, err := loadNewEducateConfigByID[educateNumericConfig](category, id)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("%s/%d: missing condition numeric configuration", category, id)
	}
	if config.Character != state.Info.GetId() {
		return fmt.Errorf("%s/%d: condition belongs to character %d, current character %d", category, id, config.Character, state.Info.GetId())
	}
	return nil
}
