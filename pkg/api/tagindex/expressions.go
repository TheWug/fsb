package tagindex

import (
    "bytes"
    "bufio"
    "fmt"
    "io"
    "io/ioutil"
    "strings"
    "unicode"
)

type TagExpression interface {
    SerializeReal(buf *bytes.Buffer, index *int, tokens map[string]string)
}

func Serialize(this TagExpression) (string, map[string]string) {
    var b bytes.Buffer
    var token int
    tokens := make(map[string]string)
    this.SerializeReal(&b, &token, tokens)
    return b.String(), tokens
}

type TETag struct {
    tag    string
    negate bool
}

func (this *TETag) SerializeReal(buf *bytes.Buffer, index *int, tokens map[string]string) {
    expects := ""
    if this.negate {
        expects = "NOT"
    }

    token := fmt.Sprintf("token%d", *index)
    *index++
    tokens[token] = this.tag
    buf.WriteString(fmt.Sprintf("((SELECT {{.tag_id}} FROM {{.tag_index}} WHERE {{.tag_name}} = {{.%s}}) IN (SELECT * FROM {{.temp}}) IS %s TRUE)", token, expects))
}

type TENegate struct {
    sub_exp TagExpression
}

func (this *TENegate) SerializeReal(buf *bytes.Buffer, index *int, tokens map[string]string) {
    buf.WriteString("(NOT (")
    this.sub_exp.SerializeReal(buf, index, tokens)
    buf.WriteString("))")
}

type TEAnd struct {
    sub_exp_1, sub_exp_2 TagExpression
}

func (this *TEAnd) SerializeReal(buf *bytes.Buffer, index *int, tokens map[string]string) {
    buf.WriteString("((")
    this.sub_exp_1.SerializeReal(buf, index, tokens)
    buf.WriteString(") AND (")
    this.sub_exp_2.SerializeReal(buf, index, tokens)
    buf.WriteString("))")
}

type TEOr struct {
    sub_exp_1, sub_exp_2 TagExpression
}

func (this *TEOr) SerializeReal(buf *bytes.Buffer, index *int, tokens map[string]string) {
    buf.WriteString("((")
    this.sub_exp_1.SerializeReal(buf, index, tokens)
    buf.WriteString(") OR (")
    this.sub_exp_2.SerializeReal(buf, index, tokens)
    buf.WriteString("))")
}

func Tokenize(expression string) ([]string) {
    reader := bufio.NewReader(bytes.NewBuffer([]byte(expression)))
    var out []string
    var tok bytes.Buffer
    append := func() {
        b, _ := ioutil.ReadAll(&tok)
        if len(b) != 0 {
            out = append(out, string(b))
        }
    }

    escaped := false
    for {
        r, _, err := reader.ReadRune()
        if err == io.EOF {
            append()
            for i := len(out)/2-1; i >= 0; i-- {
	            opp := len(out)-1-i
	            out[i], out[opp] = out[opp], out[i]
            }
            return out
        }

        if unicode.IsSpace(r) {
            append()
            continue
        }

        if r == rune('\\') {
            escaped = true
        } else {
            if r == rune('{') || r == rune('}') {
                if !escaped {
                    append()
                }
                tok.WriteRune(r)
                if !escaped {
                    append()
                }
            } else if r == rune(',') {
                append()
                tok.WriteRune(r)
                append()
            } else {
                tok.WriteRune(r)
            }
            escaped = false
        }
    }
}

type stack []string

func (this *stack) push(elem string) {
    *this = append(*this, elem)
}

func (this *stack) pop() *string {
    if len(*this) == 0 { return nil }
    elem := (*this)[len(*this) - 1]
    *this = (*this)[:len(*this) - 1]
    return &elem
}

func Parse(tokens []string) TagExpression {
    s := stack(tokens)
    e, s := ParseExpression(s, 5)
    if len(s) != 0 { return nil }
    return e
}

func ParseExpression(pushdown stack, maxlevel int) (TagExpression, stack) {
    sub_exp, sub_pushdown := ParseSubExpression(pushdown)
    if sub_exp != nil && len(sub_pushdown) == 0 { return sub_exp, sub_pushdown }
    if sub_exp == nil {
        sub_exp, sub_pushdown = ParseLiteral(pushdown)
        if sub_exp != nil && len(sub_pushdown) == 0 { return sub_exp, sub_pushdown }
        if sub_exp == nil {
            sub_exp, sub_pushdown = ParseNegation(pushdown)
            if sub_exp != nil && len(sub_pushdown) == 0 { return sub_exp, sub_pushdown }
        }
    }

    if maxlevel == 3 || sub_exp == nil { return sub_exp, sub_pushdown }

    new_exp, new_pushdown := ParseIntersection(sub_exp, sub_pushdown)
    if new_exp != nil {
        sub_exp, sub_pushdown = new_exp, new_pushdown
    }
    if maxlevel == 4 || new_exp != nil && len(new_pushdown) == 0 { return sub_exp, sub_pushdown }

    new_exp, new_pushdown = ParseUnion(sub_exp, sub_pushdown)
    if new_exp != nil {
        sub_exp, sub_pushdown = new_exp, new_pushdown
    }
    return sub_exp, sub_pushdown
}

func ParseSubExpression(s stack) (TagExpression, stack) {
    tok := s.pop()
    if tok == nil || *tok != "{" {
        fmt.Println("subexpression: no dice")
        return nil, nil
    }
    fmt.Println("subexpression: ", *tok)
    e, ns := ParseExpression(s, 5)
    if e == nil {
        fmt.Println("subexpression: no dice (subexpression)")
        return nil, nil
    }
    tok = ns.pop()
    if tok == nil || *tok != "}" {
        fmt.Println("subexpression: no dice")
        return nil, nil
    }
    fmt.Println("subexpression: ", *tok)
    return e, ns
}

func ParseLiteral(s stack) (TagExpression, stack) {
    tok := s.pop()
    if tok == nil || *tok == "{" || *tok == "}" || *tok == "," || *tok == "-" || strings.ContainsAny(*tok, "%#*") || strings.HasPrefix(*tok, "~") {
        fmt.Println("literal: no dice")
        return nil, nil
    }
    fmt.Println("literal: ", *tok)
    if strings.HasPrefix(*tok, "-") {
        return &TETag{tag: (*tok)[1:], negate: true}, s
    } else {
        return &TETag{tag: *tok, negate: false}, s
    }
}

func ParseNegation(s stack) (TagExpression, stack) {
    tok := s.pop()
    if tok == nil || *tok != "-" {
        fmt.Println("negation: no dice")
        return nil, nil
    }
    fmt.Println("negation: ", *tok)
    e, ns := ParseExpression(s, 3)
    if e == nil {
        fmt.Println("negation: no dice (subexpression)")
        return nil, nil
    }
    return &TENegate{sub_exp: e}, ns
}

func ParseIntersection(first TagExpression, s stack) (TagExpression, stack) {
    fmt.Println("intersection: parsing subexpression")
    e, ns := ParseExpression(s, 4)
    if e == nil {
        fmt.Println("intersection: no dice (subexpression)")
        return nil, nil
    }
    return &TEAnd{sub_exp_1: first, sub_exp_2: e}, ns
}

func ParseUnion(first TagExpression, s stack) (TagExpression, stack) {
    tok := s.pop()
    if tok == nil || *tok != "," {
        fmt.Println("union: no dice")
        return nil, nil
    }
    fmt.Println("union: ", *tok)
    fmt.Println("union: parsing subexpression")
    e, ns := ParseExpression(s, 5)
    if e == nil {
        fmt.Println("union: no dice (subexpression)")
        return nil, nil
    }
    return &TEOr{sub_exp_1: first, sub_exp_2: e}, ns
}
