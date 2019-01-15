package at

import (
	"fmt"
	"strconv"
	"strings"
)

// FullParse 将字符串深度解析
func FullParse(sntext string) Attribute {
	return nil
}

// Parse 所有 Value 只解析到 String
func Parse(sntext string) Attribute {
	return (&attr{}).parse(sntext)
}

const EOF = -1

type attr struct {
	data []rune
	len  int
	pos  int
	r    rune
}

func printMapTemplate(data Attribute, depth int) {
	s := ""
	for i := 0; i != depth; i++ {
		s += "  "
	}

	s2 := s + "  "
	fmt.Println(s + "{")
	for k, v := range data {
		switch t := v.(type) {
		case Attribute:
			fmt.Printf(s2+"%s: ", k)
			print(t, depth+1)
		default:
			fmt.Printf(s2+"%s: %s,\n", k, t)
		}
	}
	fmt.Println(s + "}")
}

// TODO：：替换 panic 为 error
func (me *attr) parse(template string) Attribute {
	me.data = []rune(template)
	me.len = len(me.data)
	me.pos = -1
	me.r = EOF

	me.next()
	me.skipBlank()
	if me.r != '{' {
		panic("first element is not {")
	}
	return me.nextStruct()
}

func (me *attr) nextStruct() Attribute {
	data := make(Attribute, 0)

	// fmt.Println(string(me.r))

	for {
		me.next()
		me.skipBlank()
		if me.r == '}' || me.r == EOF {
			break
		}
		if me.r == ',' {
			continue
		}

		key := me.nextKey()
		me.next()
		me.skipBlank()
		value := me.nextValue()
		// fmt.Printf("%s: %s $%c\n", key, value, me.r)
		data[key] = value
	}

	return data
}

func (me *attr) nextValue() interface{} {
	switch me.r {
	case '"', '`':
		return me.nextString(me.r)
	case '{':
		return me.nextStruct()
	case '[':
		return me.nextArray()
	default:
		return me.nextString(',')
	}
	return nil
}

func (me *attr) nextArray() []interface{} {
	arr := make([]interface{}, 0)
	for {
		me.next()
		me.skipBlank()
		if me.r == ']' {
			break
		}
		if me.r == ',' {
			continue
		}
		value := me.nextValue()
		// fmt.Printf("%s%c", value, me.r)
		arr = append(arr, value)
	}
	// fmt.Println()
	return arr
}

func (me *attr) skipBlank() {
	for {
		switch me.r {
		case ' ', '\r', '\n', '\t', '\f', '\b', 65279:
			me.next()
			continue
		case '/':
			me.skipComment()
			continue
		default:
			break
		}
		break
	}
}

func (me *attr) next() rune {
	me.pos++
	if me.pos >= me.len {
		me.r = EOF
	} else {
		me.r = me.data[me.pos]
	}
	return me.r
}

func (me *attr) previous() rune {
	me.pos--
	if me.pos >= me.len || me.pos < 0 {
		me.r = EOF
	} else {
		me.r = me.data[me.pos]
	}
	return me.r
}

func (me *attr) skipComment() {
	me.next()
	if me.r == '/' {
		for {
			me.next()
			if me.r == '\n' {
				me.next()
				return
			} else if me.r == EOF {
				return
			}
		}
	} else if me.r == '*' {
		me.next()
		for me.r != EOF {
			if me.r == '*' {
				me.next()
				if me.r == '/' {
					me.next()
					return
				}
				continue
			}
			me.next()
		}
	} else {
		panic("invalid comment")
	}
}

// obtain next key, and last rune is ':'
func (me *attr) nextKey() string {
	p := me.pos
	if me.r < 'A' || (me.r > 'Z' && me.r < 'a') || me.r > 'z' {
		panic(fmt.Sprintf("%s, %c=%d", "首字母必须为字母", me.r, me.r))
	}
	me.next()
	for (me.r >= 'a' && me.r <= 'z') || (me.r >= 'A' && me.r <= 'Z') || (me.r >= '0' && me.r <= '9') || me.r == '_' {
		me.next()
	}
	key := string(me.data[p:me.pos])
	me.skipBlank()
	if me.r != ':' {
		panic("无效的....")
	}
	return key
}

func (me *attr) nextString(end rune) string {
	p := me.pos
	ret := ""
	quot := (end == '"' || end == '`')

	for {
		me.next()
		if quot {
			switch me.r {
			case '"', '`':
				if me.data[me.pos-1] != '\\' {
					ret = string(me.data[p+1 : me.pos])
					goto out
				} else {
					// TODO
				}

			case EOF:
				panic("unclosed string")
			}
		} else {
			switch me.r {
			case ',', '}', ']':
				if me.data[me.pos-1] != '\\' {
					ret = strings.TrimSpace(string(me.data[p:me.pos]))
					me.previous()
					goto out
				} else {
					// TODO
				}

			case EOF:
				panic("unclosed string")
			}
		}

	}

out:
	return ret
}

func (me *attr) nextValue2(end rune) interface{} {
	// TODO hasSpecial := false
	p := me.pos
	ret := ""
	// sp := make([]rune, 5)
	digit := true
	isFloat := false
	isString := (me.r == '"' || me.r == '`')

	for {
		me.next()
		if me.r == '\\' {
			// TODO
		}

		if me.r == '"' || me.r == '`' {
			ret = string(me.data[p+1 : me.pos])
			goto out
		}

		if me.r == ',' || me.r == '}' || me.r == ']' {
			ret = strings.TrimSpace(string(me.data[p:me.pos]))
			me.previous()
			goto out
		}

		if !isString {
			if (me.r < '0' || me.r > '9') && me.r != '.' && me.r != '-' && me.r != '+' {
				digit = false
			}

			if me.r == '.' {
				if digit {
					isFloat = true
				}
			}
		}

		if me.r == EOF {
			panic("unclosed string")
		}
	}

out:
	if !isString && digit {
		if isFloat {
			f, e := strconv.ParseFloat(ret, 64)
			if e != nil {
				fmt.Println(e)
				return ret
			}
			return f
		}

		i, e := strconv.ParseInt(ret, 10, 64)
		if e != nil {
			fmt.Println(e)
			return ret
		}
		return i
	}

	return ret
}
