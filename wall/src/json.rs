//! 最小 JSON：**只实现卵配方真正用到的子集**（无第三方依赖）。
//!
//! 为什么自己写：依赖政策是「零第三方依赖优先」（`Cargo.toml`）。这个子集足够小 —— 配方是**扁平结构**
//! （对象 + 字符串/整数/字符串数组/字符串字典），不需要浮点、不需要流式、不需要大数。
//!
//! **严格即正确**（与茧壁的整体口径一致）：
//!   - 语法不对 / 尾部还有垃圾 / 出现浮点 ⇒ **报错**，不「尽力解析」；
//!   - 只认整数（契约里没有浮点字段；认不得就报错，免得把 `1.0` 当成 `1` 静默收下）。
//!
//! 将来若允许依赖，把 `parse` / `quote` 换成 `serde_json` 即可 —— **函数签名就是那道替换缝**。

use std::collections::BTreeMap;

use crate::error::Error;

/// JSON 值（本子集）。
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Value {
    /// `null`
    Null,
    /// `true` / `false`
    Bool(bool),
    /// 整数（浮点不支持：见模块说明）
    Int(i64),
    /// 字符串
    Str(String),
    /// 数组
    Arr(Vec<Value>),
    /// 对象（键序稳定：`BTreeMap` ⇒ 同一份输入每次给出同一条错误、同一种遍历顺序）
    Obj(BTreeMap<String, Value>),
}

impl Value {
    /// 取对象（非对象 ⇒ 拒绝，理由里带实际类型名 —— 报错要能定位）。
    pub fn as_obj(&self) -> Result<&BTreeMap<String, Value>, Error> {
        match self {
            Value::Obj(m) => Ok(m),
            other => Err(Error::new(&format!(
                "期望一个 JSON 对象，实得{}",
                other.kind_name()
            ))),
        }
    }

    /// 取数组。
    pub fn as_arr(&self) -> Result<&[Value], Error> {
        match self {
            Value::Arr(v) => Ok(v),
            other => Err(Error::new(&format!(
                "期望一个 JSON 数组，实得{}",
                other.kind_name()
            ))),
        }
    }

    /// 取字符串。
    pub fn as_str(&self) -> Result<&str, Error> {
        match self {
            Value::Str(s) => Ok(s),
            other => Err(Error::new(&format!(
                "期望一个 JSON 字符串，实得{}",
                other.kind_name()
            ))),
        }
    }

    /// 取整数。
    pub fn as_int(&self) -> Result<i64, Error> {
        match self {
            Value::Int(n) => Ok(*n),
            other => Err(Error::new(&format!(
                "期望一个 JSON 整数，实得{}",
                other.kind_name()
            ))),
        }
    }

    /// 对象里取一个键（非对象 / 缺键 ⇒ `None`；由调用方决定「缺」是拒绝还是可省）。
    pub fn get(&self, key: &str) -> Option<&Value> {
        match self {
            Value::Obj(m) => m.get(key),
            _ => None,
        }
    }

    /// 类型名（只用在本端自己的报错里，不假设外部取值）。
    fn kind_name(&self) -> &'static str {
        match self {
            Value::Null => "null",
            Value::Bool(_) => "布尔值",
            Value::Int(_) => "整数",
            Value::Str(_) => "字符串",
            Value::Arr(_) => "数组",
            Value::Obj(_) => "对象",
        }
    }
}

/// 把任意字符串编成一个**合法的 JSON 字符串字面量**（含两端引号）。
///
/// 逐字符转义：`"` `\` 与控制字符一律 `\uXXXX`（控制字符按规范不能原样出现）；
/// 其余字符原样输出（UTF-8 直出即可，JSON 允许）。
pub fn quote(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for ch in s.chars() {
        match ch {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            c if (c as u32) < 0x20 => out.push_str(&format!("\\u{:04x}", c as u32)),
            c => out.push(c),
        }
    }
    out.push('"');
    out
}

/// 解析一段 JSON 文本（严格：见模块说明）。
pub fn parse(src: &str) -> Result<Value, Error> {
    let chars: Vec<char> = src.chars().collect();
    let mut p = Parser { c: &chars, i: 0 };
    p.skip_ws();
    let v = p.value()?;
    p.skip_ws();
    if p.i != p.c.len() {
        return Err(Error::new(&format!(
            "JSON 尾部还有多余内容（第 {} 个字符起）——拒绝：不接受「尽力解析」",
            p.i + 1
        )));
    }
    Ok(v)
}

struct Parser<'a> {
    c: &'a [char],
    i: usize,
}

impl Parser<'_> {
    fn peek(&self) -> Option<char> {
        self.c.get(self.i).copied()
    }

    fn skip_ws(&mut self) {
        while let Some(ch) = self.peek() {
            if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
                self.i += 1;
            } else {
                break;
            }
        }
    }

    fn value(&mut self) -> Result<Value, Error> {
        match self.peek() {
            None => Err(Error::new("JSON 意外结束（期望一个值）")),
            Some('{') => self.object(),
            Some('[') => self.array(),
            Some('"') => Ok(Value::Str(self.string()?)),
            Some('t') => self.literal("true", Value::Bool(true)),
            Some('f') => self.literal("false", Value::Bool(false)),
            Some('n') => self.literal("null", Value::Null),
            Some(ch) if ch == '-' || ch.is_ascii_digit() => self.number(),
            Some(ch) => Err(Error::new(&format!("JSON 里出现了认不得的字符 {ch:?}"))),
        }
    }

    fn literal(&mut self, word: &str, v: Value) -> Result<Value, Error> {
        for want in word.chars() {
            if self.peek() != Some(want) {
                return Err(Error::new(&format!("JSON 字面量 {word} 不完整")));
            }
            self.i += 1;
        }
        Ok(v)
    }

    fn object(&mut self) -> Result<Value, Error> {
        self.i += 1; // '{'
        let mut m = BTreeMap::new();
        self.skip_ws();
        if self.peek() == Some('}') {
            self.i += 1;
            return Ok(Value::Obj(m));
        }
        loop {
            self.skip_ws();
            let k = self.string()?;
            self.skip_ws();
            if self.peek() != Some(':') {
                return Err(Error::new(&format!("JSON 对象里键 {k:?} 之后缺少 `:`")));
            }
            self.i += 1;
            self.skip_ws();
            let v = self.value()?;
            // 重复键 ⇒ 拒绝：静默后一个盖前一个，与「说不清哪一条生效」是同一类故障。
            if m.insert(k.clone(), v).is_some() {
                return Err(Error::new(&format!(
                    "JSON 对象里键 {k:?} 出现了两次——拒绝：不许静默覆盖"
                )));
            }
            self.skip_ws();
            match self.peek() {
                Some(',') => {
                    self.i += 1;
                }
                Some('}') => {
                    self.i += 1;
                    return Ok(Value::Obj(m));
                }
                _ => return Err(Error::new("JSON 对象里期望 `,` 或 `}`")),
            }
        }
    }

    fn array(&mut self) -> Result<Value, Error> {
        self.i += 1; // '['
        let mut v = Vec::new();
        self.skip_ws();
        if self.peek() == Some(']') {
            self.i += 1;
            return Ok(Value::Arr(v));
        }
        loop {
            self.skip_ws();
            v.push(self.value()?);
            self.skip_ws();
            match self.peek() {
                Some(',') => {
                    self.i += 1;
                }
                Some(']') => {
                    self.i += 1;
                    return Ok(Value::Arr(v));
                }
                _ => return Err(Error::new("JSON 数组里期望 `,` 或 `]`")),
            }
        }
    }

    fn string(&mut self) -> Result<String, Error> {
        if self.peek() != Some('"') {
            return Err(Error::new("JSON 期望一个字符串（键名或值）"));
        }
        self.i += 1;
        let mut out = String::new();
        loop {
            let ch = self
                .peek()
                .ok_or_else(|| Error::new("JSON 字符串没有收尾的引号"))?;
            self.i += 1;
            match ch {
                '"' => return Ok(out),
                '\\' => {
                    let e = self
                        .peek()
                        .ok_or_else(|| Error::new("JSON 转义序列不完整"))?;
                    self.i += 1;
                    match e {
                        '"' => out.push('"'),
                        '\\' => out.push('\\'),
                        '/' => out.push('/'),
                        'b' => out.push('\u{0008}'),
                        'f' => out.push('\u{000c}'),
                        'n' => out.push('\n'),
                        'r' => out.push('\r'),
                        't' => out.push('\t'),
                        'u' => {
                            let mut code: u32 = 0;
                            for _ in 0..4 {
                                let h = self
                                    .peek()
                                    .ok_or_else(|| Error::new("JSON \\u 转义不足四位"))?;
                                let d = h
                                    .to_digit(16)
                                    .ok_or_else(|| Error::new("JSON \\u 转义里有非十六进制字符"))?;
                                code = code * 16 + d;
                                self.i += 1;
                            }
                            let c = char::from_u32(code).ok_or_else(|| {
                                Error::new("JSON \\u 转义不是合法码位（代理对不在本子集）")
                            })?;
                            out.push(c);
                        }
                        other => {
                            return Err(Error::new(&format!("JSON 里认不得的转义 \\{other}")));
                        }
                    }
                }
                c if (c as u32) < 0x20 => {
                    return Err(Error::new("JSON 字符串里有裸控制字符——拒绝（必须转义）"));
                }
                c => out.push(c),
            }
        }
    }

    fn number(&mut self) -> Result<Value, Error> {
        let start = self.i;
        if self.peek() == Some('-') {
            self.i += 1;
        }
        let mut digits = 0;
        while let Some(ch) = self.peek() {
            if ch.is_ascii_digit() {
                digits += 1;
                self.i += 1;
            } else {
                break;
            }
        }
        if digits == 0 {
            return Err(Error::new("JSON 数字没有整数部分"));
        }
        // 浮点 / 指数：本子集不支持 ⇒ 明确拒绝（不许把 1.0 静默当成 1）。
        if matches!(self.peek(), Some('.') | Some('e') | Some('E')) {
            return Err(Error::new(
                "JSON 数字是浮点或指数形态——本端只认整数（契约里没有浮点字段）",
            ));
        }
        let text: String = self.c[start..self.i].iter().collect();
        text.parse::<i64>()
            .map(Value::Int)
            .map_err(|_| Error::new(&format!("JSON 整数 {text} 超出本端范围")))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn quote_escapes_control_and_quotes() {
        assert_eq!(quote("a\"b\\c\nd"), "\"a\\\"b\\\\c\\nd\"");
        assert_eq!(quote("\u{0001}"), "\"\\u0001\"");
    }

    #[test]
    fn parse_nested_and_ordered() {
        let v = parse(r#"{"b":1,"a":["x",true,null]}"#).expect("应能解析");
        assert_eq!(v.get("b").and_then(|x| x.as_int().ok()), Some(1));
        let arr = v.get("a").expect("有 a");
        assert_eq!(arr.as_arr().expect("是数组").len(), 3);
    }

    #[test]
    fn parse_refuses_float_trailing_and_duplicate_key() {
        // 三条负例都必须红（浮点 / 尾部垃圾 / 重复键）
        assert!(parse("1.0").is_err(), "浮点必须拒");
        assert!(parse("{} x").is_err(), "尾部垃圾必须拒");
        assert!(parse(r#"{"a":1,"a":2}"#).is_err(), "重复键必须拒");
    }
}
