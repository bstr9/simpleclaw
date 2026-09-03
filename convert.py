import sys
import polars as pl
import os

csv, out = sys.argv[1], sys.argv[2]

with open(csv) as f:
    first = f.readline()
HAS_HEADER = first.split(",")[0] == "id"

# 读取：只保留价格3列+时间戳（列子集省内存），无header时按位置重命名
df = pl.read_csv(
    csv, has_header=HAS_HEADER,
    columns=[1, 2, 3, 4] if not HAS_HEADER else None,
    new_columns=["price", "qty", "quote_qty", "timestamp"] if not HAS_HEADER else None,
    schema_overrides={"price": pl.Float64, "qty": pl.Float64,
                      "quote_qty": pl.Float64, "timestamp": pl.Int64},
)
print(f"trades: {len(df)}")

# 微秒/毫秒自适应
DIV = pl.when(pl.col("timestamp") > 10**15).then(1_000_000).otherwise(1_000)
df = df.with_columns((pl.col("timestamp") // DIV).alias("ts_sec"))

ohlcv = (
    df.sort("timestamp")
    .group_by("ts_sec", maintain_order=True)
    .agg([
        pl.col("price").first().alias("open"),
        pl.col("price").max().alias("high"),
        pl.col("price").min().alias("low"),
        pl.col("price").last().alias("close"),
        pl.col("qty").sum().alias("volume"),
        pl.col("quote_qty").sum().alias("quote_volume"),
        pl.len().alias("trade_count"),
        pl.col("timestamp").first().alias("first_trade_ms"),
        pl.col("timestamp").last().alias("last_trade_ms"),
    ])
)
print(f"1s bars: {len(ohlcv)}")

ohlcv = ohlcv.with_columns(
    pl.from_epoch(pl.col("ts_sec"), time_unit="s").cast(pl.String).alias("datetime_utc")
)
ohlcv.select(["ts_sec", "datetime_utc", "open", "high", "low", "close",
              "volume", "quote_volume", "trade_count",
              "first_trade_ms", "last_trade_ms"]).write_parquet(out)
print(f"saved {out}: {os.path.getsize(out)/1024/1024:.1f} MB")
