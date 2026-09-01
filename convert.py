import sys
import polars as pl
import os

csv, out = sys.argv[1], sys.argv[2]

df = pl.read_csv(
    csv,
    has_header=True,
    columns=[0, 1, 2, 3, 4],
    new_columns=["id", "price", "qty", "quote_qty", "timestamp_ms"],
    schema_overrides={"id": pl.Int64, "price": pl.Float64, "qty": pl.Float64,
                      "quote_qty": pl.Float64, "timestamp_ms": pl.Int64},
)
print(f"trades: {len(df)}")

df = df.with_columns((pl.col("timestamp_ms") // 1000).alias("ts_sec"))

ohlcv = (
    df.sort("timestamp_ms")
    .group_by("ts_sec", maintain_order=True)
    .agg([
        pl.col("price").first().alias("open"),
        pl.col("price").max().alias("high"),
        pl.col("price").min().alias("low"),
        pl.col("price").last().alias("close"),
        pl.col("qty").sum().alias("volume"),
        pl.col("quote_qty").sum().alias("quote_volume"),
        pl.len().alias("trade_count"),
        pl.col("timestamp_ms").first().alias("first_trade_ms"),
        pl.col("timestamp_ms").last().alias("last_trade_ms"),
    ])
)
print(f"1s bars: {len(ohlcv)}")

ohlcv = ohlcv.with_columns(
    pl.from_epoch(pl.col("ts_sec"), time_unit="s").cast(pl.String).alias("datetime_utc")
)
ohlcv.select(["ts_sec", "datetime_utc", "open", "high", "low", "close",
               "volume", "quote_volume", "trade_count",
               "first_trade_ms", "last_trade_ms"]).write_parquet(out)

import os
print(f"saved {out}: {os.path.getsize(out)/1024/1024:.1f} MB")
