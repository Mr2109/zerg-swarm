//! 卵配方（`Spec`）—— 茧壁的**唯一输入**，字段语义与 Go 侧 `hatch.Spec` 一一对应。
//!
//! **只搬「策略面」**：茧壁要用到的字段（引擎入口 / 参数 / 绑定 / 环境 / 设备 / 工作目录 / memlock）。
//! Go 侧 `hatch.Spec` 里还有 `Profile`（实测档案）等**决策面**字段，茧壁**不消费**它们，故这里没有
//! —— 缺了就当「认不得的键」忽略（见 `from_json`），**不复制一份档案校验逻辑**（复制 = 第二份真相）。
//!
//! JSON 键名口径（批 2'.1 冻结）：**snake_case**，与 Go 侧字段的语义对应关系写在每个字段上。

use std::collections::BTreeMap;

use crate::error::Error;
use crate::json::Value;
use crate::PLAN_SCHEMA_VERSION;

/// 卵配方（策略面）。
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Spec {
    /// `egg_id` —— 卵名（注册表键）。空 ⇒ 拒（身份都定不下来）。
    pub egg_id: String,
    /// `schema_version` —— 卵声明格式版本（必须 = [`PLAN_SCHEMA_VERSION`]，即 Go 侧
    /// `registry.EggSchemaVersionCurrent`；陌生版本 ⇒ 拒，不按新格式猜着跑）。
    pub schema_version: u32,
    /// `engine_path_in_space` —— 引擎在**空间内**的可执行路径（如 `/engine/bin/llama-server`）。
    pub engine_path_in_space: String,
    /// `engine_args` —— 引擎参数（由适配器产出、占位符已展开）。
    pub engine_args: Vec<String>,
    /// `engine_roots` —— 引擎自己的库/构建目录（**宿主侧**），只读挂进 `/engine`。
    pub engine_roots: Vec<String>,
    /// `weight_path` —— 权重所在宿主路径；**只用于出证**，不参与挂载（Go 侧「缺陷 12」收口后同义）。
    pub weight_path: String,
    /// `weight_files` —— 该卵点名的权重/投影/模板文件（**宿主侧绝对路径**），各落 `/models/<基名>`。
    pub weight_files: Vec<String>,
    /// `env` —— 按卵环境变量（键序由 `BTreeMap` 固定 ⇒ argv 逐字可复现）。
    pub env: BTreeMap<String, String>,
    /// `devices` —— 要暴露的设备节点；空 ⇒ 用缺省 `/dev/kfd` + `/dev/dri/renderD128`。
    pub devices: Vec<String>,
    /// `extra_ro_binds` —— 额外只读绑定，形如 `<host>:<space>`。
    pub extra_ro_binds: Vec<String>,
    /// `extra_rw_binds` —— 额外**可写**绑定，形如 `<host>:<space>`（bwrap `--bind`）。
    pub extra_rw_binds: Vec<String>,
    /// `work_dir` —— 工作目录（**空间内**路径，如 `/work`）。
    pub work_dir: String,
    /// `memlock_bytes` —— `RLIMIT_MEMLOCK` 限额（>0 才有值）。
    ///
    /// ⚠ 它**不在本 argv 内**：现有实现由**归属层**（`systemd-run --property=LimitMEMLOCK=`）下发
    /// ⇒ 茧壁只在 `note` 里如实说明「它不在本 argv 内」，**不列进 allowlist**（列进去就成了
    /// 「看着像保障、其实本件没执行」—— §6.9 同一精神）。
    pub memlock_bytes: i64,
}

/// 空间内权重落点根（与 Go 侧 `hatch.spaceModelsDir` / `backend.spaceWeightsDir` **同值**：
/// 跨包、跨语言契约，改一处必须同步改另两处）。
pub const SPACE_MODELS_DIR: &str = "/models";

impl Spec {
    /// 从解析出的 JSON 值构造配方（fail-closed：**必需字段缺 ⇒ 拒**；认不得的键忽略）。
    ///
    /// 「认不得的键忽略」是有意的（与 Go 侧 `contract.Validate` 的「未知字段必须拒」不同口径）：
    /// 配方里会带茧壁**不消费**的决策面字段（如实测档案），逐字拒绝会把好配方挡在门外；
    /// 而「少一个必需键」必须拒 —— 那才是会孵出「挂错东西」的形态。
    pub fn from_json(v: &Value) -> Result<Self, Error> {
        let obj = v.as_obj()?;
        // 版本先取出来单独看一眼：负数会被 `as u32` 绕成一极大值，报错要说清是「值不合法」。
        let raw_version = req_int(obj, "schema_version")?;
        if !(0..=u32::MAX as i64).contains(&raw_version) {
            return Err(Error::new(&format!(
                "schema_version={raw_version} 超出合法范围——拒"
            )));
        }
        Ok(Spec {
            egg_id: req_str(obj, "egg_id")?,
            schema_version: raw_version as u32,
            engine_path_in_space: req_str(obj, "engine_path_in_space")?,
            engine_args: opt_str_list(obj, "engine_args")?,
            engine_roots: opt_str_list(obj, "engine_roots")?,
            weight_path: opt_str(obj, "weight_path")?.unwrap_or_default(),
            weight_files: opt_str_list(obj, "weight_files")?,
            env: opt_str_map(obj, "env")?,
            devices: opt_str_list(obj, "devices")?,
            extra_ro_binds: opt_str_list(obj, "extra_ro_binds")?,
            extra_rw_binds: opt_str_list(obj, "extra_rw_binds")?,
            work_dir: opt_str(obj, "work_dir")?.unwrap_or_default(),
            memlock_bytes: opt_int(obj, "memlock_bytes")?.unwrap_or(0),
        })
    }

    /// 校验（fail-closed，照 Go 侧 `hatch.Spec.Validate` 的口径搬「策略面」那几条）。
    ///
    /// 搬了哪些：身份 / 版本 / 引擎路径（空间内绝对）/ 权重至少声明一处 / 工作目录是空间内路径 /
    /// 绑定形式与落点硬边界 / 权重声明 ↔ 绑定当契约核 / 环境变量名合法。
    /// 没搬哪些：实测档案（`Profile`）—— 那是**决策面**，Go 侧已核，茧壁不复制（见模块说明）。
    pub fn validate(&self) -> Result<(), Error> {
        if self.egg_id.trim().is_empty() {
            return Err(Error::new(
                "配方缺 egg_id——拒：身份定不下来，日志与落点都无从定位",
            ));
        }
        if self.schema_version != PLAN_SCHEMA_VERSION {
            return Err(Error::new(&format!(
                "卵声明格式版本 {} 认不得（本端认得 {}）——拒：不按新格式猜着跑",
                self.schema_version, PLAN_SCHEMA_VERSION
            )));
        }
        if self.engine_path_in_space.trim().is_empty() {
            return Err(Error::new(
                "配方缺 engine_path_in_space（引擎在空间内的路径）——拒",
            ));
        }
        if !self.engine_path_in_space.starts_with('/') {
            return Err(Error::new(&format!(
                "引擎路径必须是**空间内**的绝对路径（如 /engine/bin/llama-server），实得 {:?}——拒",
                self.engine_path_in_space
            )));
        }
        if self.weight_files.is_empty() && self.weight_path.trim().is_empty() {
            return Err(Error::new(
                "配方缺权重（weight_files 与 weight_path 都是空）——拒：不孵一枚空手进来的卵",
            ));
        }
        if !self.work_dir.is_empty() && !self.work_dir.starts_with('/') {
            return Err(Error::new(&format!(
                "工作目录必须是**空间内**的绝对路径（如 /work），实得 {:?}（宿主侧形态）——拒：\
                 宿主目录要经 extra_rw_binds 绑到该落点上",
                self.work_dir
            )));
        }
        validate_binds("只读", &self.extra_ro_binds)?;
        validate_binds("可写", &self.extra_rw_binds)?;
        reject_models_dir_mounts(self)?;
        validate_weight_files(self)?;
        validate_env(&self.env)?;
        Ok(())
    }
}

/// 取必需字符串（缺 / 空串 / 非字符串 ⇒ 拒）。
fn req_str(obj: &BTreeMap<String, Value>, key: &str) -> Result<String, Error> {
    let v = obj
        .get(key)
        .ok_or_else(|| Error::new(&format!("配方缺必需键 {key:?}——拒（不取默认值）")))?;
    let s = v.as_str()?.to_string();
    if s.trim().is_empty() {
        return Err(Error::new(&format!(
            "配方里 {key:?} 是空串——拒（拿不到就报错，不猜）"
        )));
    }
    Ok(s)
}

/// 取必需整数。
fn req_int(obj: &BTreeMap<String, Value>, key: &str) -> Result<i64, Error> {
    obj.get(key)
        .ok_or_else(|| Error::new(&format!("配方缺必需键 {key:?}——拒（不取默认值）")))?
        .as_int()
}

/// 取可选字符串（键在时必须是字符串；键不在 ⇒ None）。
fn opt_str(obj: &BTreeMap<String, Value>, key: &str) -> Result<Option<String>, Error> {
    match obj.get(key) {
        None | Some(Value::Null) => Ok(None),
        Some(v) => Ok(Some(v.as_str()?.to_string())),
    }
}

/// 取可选整数。
fn opt_int(obj: &BTreeMap<String, Value>, key: &str) -> Result<Option<i64>, Error> {
    match obj.get(key) {
        None | Some(Value::Null) => Ok(None),
        Some(v) => Ok(Some(v.as_int()?)),
    }
}

/// 取可选字符串数组（逐个元素必须是字符串：混进一个非字符串 ⇒ 拒，不跳过去）。
fn opt_str_list(obj: &BTreeMap<String, Value>, key: &str) -> Result<Vec<String>, Error> {
    match obj.get(key) {
        None | Some(Value::Null) => Ok(Vec::new()),
        Some(v) => {
            let mut out = Vec::new();
            for item in v.as_arr()? {
                out.push(item.as_str()?.to_string());
            }
            Ok(out)
        }
    }
}

/// 取可选字符串字典（键与值都必须是字符串）。
fn opt_str_map(
    obj: &BTreeMap<String, Value>,
    key: &str,
) -> Result<BTreeMap<String, String>, Error> {
    match obj.get(key) {
        None | Some(Value::Null) => Ok(BTreeMap::new()),
        Some(v) => {
            let mut out = BTreeMap::new();
            for (k, val) in v.as_obj()? {
                out.insert(k.clone(), val.as_str()?.to_string());
            }
            Ok(out)
        }
    }
}

/// 逐条校验绑定形式 `<host>:<space>`（两条路径都不许是空白；口径与 Go 侧 `splitBind` 一致）。
pub fn validate_binds(kind: &str, binds: &[String]) -> Result<(), Error> {
    for b in binds {
        split_bind(kind, b)?;
    }
    Ok(())
}

/// 把一条绑定拆成 (宿主路径, 空间内路径)；形式不对 ⇒ 拒（绝不猜一个落点）。
pub fn split_bind<'a>(kind: &str, b: &'a str) -> Result<(&'a str, &'a str), Error> {
    match b.split_once(':') {
        Some((host, space)) if !host.trim().is_empty() && !space.trim().is_empty() => {
            Ok((host, space))
        }
        _ => Err(Error::new(&format!(
            "额外{kind}绑定必须是 <host>:<space> 形式，实得 {b:?}——拒"
        ))),
    }
}

/// 硬边界：**任何**绑定的落点都不许是 `/models` **本身**；**可写**绑定还不许落在 `/models/…` 之下。
///
/// 与 Go 侧 `rejectModelsDirMounts` 同口径（缺陷 12 的机器判据）：整目录挂载会把同目录的别的模型
/// 一起带进空间，且 `/models` 一旦成了只读挂载，逐文件绑定就落不进去。
pub fn reject_models_dir_mounts(s: &Spec) -> Result<(), Error> {
    for (kind, binds) in [("只读", &s.extra_ro_binds), ("可写", &s.extra_rw_binds)] {
        for b in binds {
            let (_, space) = split_bind(kind, b)?;
            let clean = clean_path(space);
            if clean == SPACE_MODELS_DIR {
                return Err(Error::new(&format!(
                    "额外{kind}绑定 {b:?} 把落点定在 {SPACE_MODELS_DIR} **本身**（整目录挂载）——拒：\
                     它会把同目录的别的模型一起带进空间（缺陷 12）"
                )));
            }
            if kind == "可写" && clean.starts_with(&format!("{SPACE_MODELS_DIR}/")) {
                return Err(Error::new(&format!(
                    "可写绑定 {b:?} 把落点定在 {SPACE_MODELS_DIR} 之下——拒：该落点上的权重会被引擎改写"
                )));
            }
        }
    }
    Ok(())
}

/// 权重声明（`weight_files`）↔ 绑定（`extra_ro_binds`）当**契约**核：声明了却没挂、挂的不是同一个、
/// 落点撞车 —— 一律拒（真机上都是最贵的那类静默故障：引擎读不到权重，而核验看不见差异）。
pub fn validate_weight_files(s: &Spec) -> Result<(), Error> {
    if s.weight_files.is_empty() {
        return Ok(());
    }
    // 空间内落点 → 已出现的只读绑定宿主
    let mut bound_at: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for b in &s.extra_ro_binds {
        let (host, space) = split_bind("只读", b)?;
        bound_at
            .entry(clean_path(space))
            .or_default()
            .push(host.to_string());
    }
    let mut seen: BTreeMap<String, String> = BTreeMap::new();
    for (i, raw) in s.weight_files.iter().enumerate() {
        let decl = raw.trim();
        if decl.is_empty() {
            return Err(Error::new(&format!(
                "权重文件清单（weight_files）第 {} 项是空串——拒",
                i + 1
            )));
        }
        if !decl.starts_with('/') {
            return Err(Error::new(&format!(
                "权重文件 {decl:?} 不是宿主侧绝对路径——拒（逐文件只读绑定要求宿主绝对路径）"
            )));
        }
        let base = path_base(decl);
        if base.is_empty() || base == "." || base == ".." || base == "/" {
            return Err(Error::new(&format!(
                "权重文件 {decl:?} 推不出基名——拒（落点 {SPACE_MODELS_DIR}/<基名> 定不下来）"
            )));
        }
        let landing = format!("{SPACE_MODELS_DIR}/{base}");
        if let Some(prev) = seen.insert(landing.clone(), decl.to_string()) {
            return Err(Error::new(&format!(
                "权重文件 {prev:?} 与 {decl:?} 的空间内落点都是 {landing:?}——拒：落点撞车等于有一份静默不见"
            )));
        }
        let hosts = bound_at.get(&landing).cloned().unwrap_or_default();
        match hosts.len() {
            0 => {
                return Err(Error::new(&format!(
                    "权重文件 {decl:?} 没有对应的只读绑定（缺 {:?}）——拒：声明了却没挂进空间",
                    format!("{decl}:{landing}")
                )));
            }
            1 => {
                if clean_path(&hosts[0]) != clean_path(decl) {
                    return Err(Error::new(&format!(
                        "空间内落点 {landing:?} 上挂的是 {:?}，声明里写的是 {decl:?}——拒：挂的文件与声明报的不是同一个",
                        hosts[0]
                    )));
                }
            }
            _ => {
                return Err(Error::new(&format!(
                    "空间内落点 {landing:?} 上有多条只读绑定（{hosts:?}）——拒：说不清哪一条生效"
                )));
            }
        }
    }
    Ok(())
}

/// 环境变量表校验：**名字**必须是合法 POSIX 名（它要由包装 exec 的 `export` 承接，名字非法会让整条
/// 包装脚本走偏），**值**里不许有 NUL（execve 不可能接受）。
pub fn validate_env(env: &BTreeMap<String, String>) -> Result<(), Error> {
    for (k, v) in env {
        if !is_env_name(k) {
            return Err(Error::new(&format!(
                "环境变量名 {k:?} 不是合法 POSIX 名（^[A-Za-z_][A-Za-z0-9_]*$）——拒：\
                 名字非法会让整条包装脚本走偏"
            )));
        }
        if v.contains('\0') {
            return Err(Error::new(&format!(
                "环境变量 {k} 的值含 NUL——execve 不可能接受，拒"
            )));
        }
    }
    Ok(())
}

/// 合法 POSIX 环境变量名（不引 regex：这条规则短到用不上整台机器）。
fn is_env_name(s: &str) -> bool {
    let mut it = s.chars();
    match it.next() {
        Some(c) if c.is_ascii_alphabetic() || c == '_' => {}
        _ => return false,
    }
    it.all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// 路径基名（POSIX 口径，等价于 Go 的 `filepath.Base(filepath.Clean(p))`）。
pub fn path_base(p: &str) -> String {
    let clean = clean_path(p);
    match clean.rfind('/') {
        Some(i) => clean[i + 1..].to_string(),
        None => clean,
    }
}

/// 路径清理（POSIX 绝对路径口径，等价于 Go 的 `filepath.Clean` 在本仓用到的那部分语义）：
/// 折叠重复 `/`、去掉 `.`、把 `..` 回退一层（根上回退仍是根）。
///
/// 为什么必须有一个：判「挂的是不是同一个文件」用**清理后比较**而不是逐字比较 ——
/// 逐字比较会把「同一个文件的两种写法」判成不符，于是误拒一枚好卵（比漏判更糟）。
pub fn clean_path(p: &str) -> String {
    let abs = p.starts_with('/');
    let mut out: Vec<&str> = Vec::new();
    for seg in p.split('/') {
        match seg {
            "" | "." => {}
            ".." => {
                out.pop();
            }
            s => out.push(s),
        }
    }
    let joined = out.join("/");
    if abs {
        format!("/{joined}")
    } else {
        joined
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn clean_path_matches_posix_expectations() {
        assert_eq!(clean_path("/a//b/./c"), "/a/b/c");
        assert_eq!(clean_path("/a/b/../c"), "/a/c");
        assert_eq!(clean_path("/.."), "/");
        assert_eq!(path_base("/a/b/model.gguf"), "model.gguf");
    }

    #[test]
    fn env_name_rule() {
        assert!(is_env_name("LD_LIBRARY_PATH"));
        assert!(is_env_name("_x1"));
        assert!(!is_env_name("1BAD"));
        assert!(!is_env_name("BAD-NAME"));
        assert!(!is_env_name(""));
    }
}
