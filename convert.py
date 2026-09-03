import sys
import polars as pl
import os

csv, out = sys.argv[1], sys.argv[2]

# 分块流式处理：每块500万行，避免13GB CSV全量进内存
# OHLCV按秒聚合可分块合并：open=first块的first, close=最后块的last, high=max, low=min
reader = pl.read_csv_batched(
    csv,
    has_header=True,
    schema_overrides={"id": pl.Int64, "price": pl.Float64, "qty": pl.Float64,
                      "quote_qty": pl.Float64, "timestamp": pl.Int64},
    batch_size=2_000_000,
)

chunks = []
while True:
    b = reader.next_batches(5)
    if b is None or len(b) == 0:
        break
    for df in b:
        df = df.select(["price", "qty", "quote_qty", "timestamp"])
        agg = (
            df.with_columns((pl.col("timestamp") // 1000).alias("ts_sec"))
            .sort("timestamp")
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
        chunks.append(agg)

# 合并所有块：同一个ts_sec可能跨块出现，需要二次聚合
big = pl.concat(chunks)
final = (
    big.sort("ts_sec")
    .group_by("ts_sec", maintain_order=True)
    .agg([
        pl.col("open").first(),
        pl.col("high").max(),
        pl.col("low").min(),
        pl.col("close").last(),
        pl.col("volume").sum(),
        pl.col("quote_volume").sum(),
        pl.col("trade_count").sum(),
        pl.col("first_trade_ms").first(),
        pl.col("last_trade_ms").last(),
    ])
)
print(f"1s bars: {len(final)}")

final = final.with_columns(
    pl.from_epoch(pl.col("ts_sec"), time_unit="s").cast(pl.String).alias("datetime_utc")
)
final.select(["ts_sec", "datetime_utc", "open", "high", "low", "close",
              "volume", "quote_volume", "trade_count",
              "first_trade_ms", "last_trade_ms"]).write_parquet(out)
print(f"saved {out}: {os.path.getsize(out)/1024/1024:.1f} MB")
