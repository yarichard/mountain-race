import json
import re
import math
from difflib import SequenceMatcher
from itertools import accumulate

import pandas as pd
import plotly.express as px
import plotly.graph_objects as go
from tqdm.auto import tqdm
from IPython.display import clear_output


GREEN = "\033[92m"
YELLOW = "\033[93m"
RED = "\033[91m"
RESET = "\033[0m"
COLOR_MAP = {"red": RED, "orange": YELLOW, "green": GREEN}

DEFAULT_SIZE = 200


def parse_json(value: str) -> list[dict] | None:
    """Extract and parse a JSON array from raw model output."""
    if isinstance(value, list):
        return value
    try:
        return json.loads(value)
    except Exception:
        pass
    match = re.search(r"\[.*\]", value, re.DOTALL)
    if match:
        try:
            return json.loads(match.group())
        except Exception:
            pass
    return None


def _name_similarity(a: str, b: str) -> float:
    return SequenceMatcher(None, a.lower().strip(), b.lower().strip()).ratio()


def _match_items(pred: list[dict], truth: list[dict], threshold: float = 0.6) -> tuple[int, int, int]:
    """Greedy name-based matching. Returns (tp, fp, fn)."""
    matched = set()
    tp = 0
    for p in pred:
        p_name = p.get("name", "")
        best, best_j = 0.0, -1
        for j, t in enumerate(truth):
            if j in matched:
                continue
            score = _name_similarity(p_name, t.get("name", ""))
            if score > best:
                best, best_j = score, j
        if best >= threshold:
            tp += 1
            matched.add(best_j)
    return tp, len(pred) - tp, len(truth) - tp


def _f1(tp: int, fp: int, fn: int) -> tuple[float, float, float]:
    precision = tp / (tp + fp) if (tp + fp) > 0 else 0.0
    recall = tp / (tp + fn) if (tp + fn) > 0 else 0.0
    f1 = (2 * precision * recall / (precision + recall)) if (precision + recall) > 0 else 0.0
    return f1, precision, recall


class JsonTester:
    def __init__(self, predictor, data, title=None, size=DEFAULT_SIZE):
        self.predictor = predictor
        self.data = data
        self.title = title or self.make_title(predictor)
        self.size = min(size, len(data))
        self.titles: list[str] = []
        self.f1s: list[float] = []
        self.precisions: list[float] = []
        self.recalls: list[float] = []
        self.valid_json: list[bool] = []
        self.colors: list[str] = []

    @staticmethod
    def make_title(predictor) -> str:
        return predictor.__name__.replace("__", ".").replace("_", " ").title().replace("Gpt", "GPT")

    def color_for(self, f1: float) -> str:
        if f1 >= 0.8:
            return "green"
        elif f1 >= 0.5:
            return "orange"
        else:
            return "red"

    def run_datapoint(self, i: int):
        datapoint = self.data[i]
        raw = self.predictor(datapoint)

        pred = parse_json(raw)
        is_valid = pred is not None
        pred = pred or []

        truth_raw = datapoint["completion"]
        truth = json.loads(truth_raw) if isinstance(truth_raw, str) else truth_raw

        tp, fp, fn = _match_items(pred, truth)
        f1, precision, recall = _f1(tp, fp, fn)
        color = self.color_for(f1)

        pieces = datapoint["prompt"].split("Gear description:")
        snippet = pieces[1].strip() if len(pieces) > 1 else datapoint["prompt"]
        title = snippet[:40] + "..." if len(snippet) > 40 else snippet

        return title, f1, precision, recall, is_valid, color

    def chart(self, title: str):
        df = pd.DataFrame({
            "f1": self.f1s,
            "precision": self.precisions,
            "recall": self.recalls,
            "title": self.titles,
            "color": self.colors,
        })
        df["hover"] = [
            f"{t}<br>F1={f:.2f}  P={p:.2f}  R={r:.2f}"
            for t, f, p, r in zip(df["title"], df["f1"], df["precision"], df["recall"])
        ]

        fig = px.histogram(
            df,
            x="f1",
            color="color",
            color_discrete_map={"green": "green", "orange": "orange", "red": "red"},
            title=title,
            labels={"f1": "F1 Score"},
            nbins=20,
            width=800,
            height=400,
        )
        fig.update_layout(bargap=0.1, showlegend=False, xaxis_range=[0, 1])
        fig.show()

    def error_trend_chart(self):
        errors = [1.0 - f for f in self.f1s]
        n = len(errors)
        x = list(range(1, n + 1))

        running_sums = list(accumulate(errors))
        running_means = [s / i for s, i in zip(running_sums, x)]

        running_squares = list(accumulate(e * e for e in errors))
        running_stds = [
            math.sqrt((sq / i) - (m ** 2)) if i > 1 else 0
            for i, sq, m in zip(x, running_squares, running_means)
        ]
        ci = [1.96 * (sd / math.sqrt(i)) if i > 1 else 0 for i, sd in zip(x, running_stds)]

        final_mean, final_ci = running_means[-1], ci[-1]
        title = f"{self.title} – Avg Error (1−F1): {final_mean:.3f} ± {final_ci:.3f}"

        upper = [m + c for m, c in zip(running_means, ci)]
        lower = [m - c for m, c in zip(running_means, ci)]

        fig = go.Figure()
        fig.add_trace(go.Scatter(
            x=x + x[::-1],
            y=upper + lower[::-1],
            fill="toself",
            fillcolor="rgba(128,128,128,0.2)",
            line=dict(color="rgba(255,255,255,0)"),
            hoverinfo="skip",
            showlegend=False,
        ))
        fig.add_trace(go.Scatter(
            x=x,
            y=running_means,
            mode="lines",
            line=dict(width=3, color="firebrick"),
            name="Cumulative Avg Error",
            customdata=list(zip(ci)),
            hovertemplate="n=%{x}<br>Avg 1−F1=%{y:.3f}<br>±95% CI=%{customdata[0]:.3f}<extra></extra>",
        ))
        fig.update_layout(
            title=title,
            xaxis_title="Number of Datapoints",
            yaxis_title="Error (1 − F1)",
            width=800,
            height=300,
            template="plotly_white",
            showlegend=False,
        )
        fig.show()

    def report(self):
        avg_f1 = sum(self.f1s) / len(self.f1s)
        avg_p = sum(self.precisions) / len(self.precisions)
        avg_r = sum(self.recalls) / len(self.recalls)
        json_rate = sum(self.valid_json) / len(self.valid_json) * 100
        title = (
            f"{self.title} results<br>"
            f"<b>F1:</b> {avg_f1:.3f}  "
            f"<b>Precision:</b> {avg_p:.3f}  "
            f"<b>Recall:</b> {avg_r:.3f}  "
            f"<b>Valid JSON:</b> {json_rate:.1f}%"
        )
        self.error_trend_chart()
        self.chart(title)

    def run(self):
        for i in tqdm(range(self.size)):
            title, f1, precision, recall, is_valid, color = self.run_datapoint(i)
            self.titles.append(title)
            self.f1s.append(f1)
            self.precisions.append(precision)
            self.recalls.append(recall)
            self.valid_json.append(is_valid)
            self.colors.append(color)
            print(f"{COLOR_MAP[color]}F1={f1:.2f} ", end="")
        clear_output(wait=True)
        self.report()


def evaluate(function, data, size=DEFAULT_SIZE):
    JsonTester(function, data, size=size).run()