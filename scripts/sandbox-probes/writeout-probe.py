import os, json
# 写空间外探针 —— 「授权级封闭」（`enclosed.os`）的**能失败**那一半：
#   ① 空间内（`PROBE_DIR`，即声明里放行的那一块）**必须能写**（一封到底不是目标）；
#   ② 空间外（`PROBE_OUTSIDE`）**必须写不进去** —— 策略若退化成全局 file-write*，这格就变成 OPEN。
# 两条都必须有基线对照（基线：空间内 ok + 空间外 OPEN），否则没有区分度 ⇒ 判据自己要先红。
w = os.environ["PROBE_DIR"]
outside = os.environ["PROBE_OUTSIDE"]
os.makedirs(w, exist_ok=True)
r = {}
try:
    open(os.path.join(w, "inside.txt"), "w").write("x"); r["write_inside"] = "ok"
except Exception as e: r["write_inside"] = "FAIL:%s" % type(e).__name__
try:
    open(outside, "w").write("x"); r["write_outside"] = "OPEN"
except Exception as e: r["write_outside"] = "BLOCKED:%s" % type(e).__name__
r["write_outside_target"] = outside
print(json.dumps(r))
