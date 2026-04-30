import json

from pydantic import BaseModel
from datasets import Dataset, DatasetDict, load_dataset
from typing import Optional, Self
from transformers import AutoTokenizer

SYSTEM_PROMPT = """You are a mountain climbing equipment assistant. Parse the following gear description and return a JSON array. Each element must have exactly three fields:
	- "name": equipment name (string, in french)
	- "quantity": number needed (integer, 1 if unspecified)
	- "notes": "optional" or "mandatory" (translated in french), plus any relevant detail (string, in french)
The name of these equipments are related with the mountain activities. You should only point out personal equipment, for instance quickdraws or rope.
You should include only equipment you're absolutely sure about. Output ONLY the JSON array, no explanation."""

tokenizer = AutoTokenizer.from_pretrained("meta-llama/Llama-3.2-3B-Instruct")

class Equipment(BaseModel):
    """
    An Equipment is a json array of equipement needed for a given route
    """

    id: int
    gear: str
    equipment: list[dict]
    prompt: Optional[str] = None
    completion: Optional[str] = None

    def make_prompt(self):

        messages = [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": f"Gear description:\n {self.gear}"},
        ]

        self.prompt = tokenizer.apply_chat_template(
            messages,
            tokenize=False,
            add_generation_prompt=True  # adds the <|start_header_id|>assistant<|end_header_id|> opener
        )
        self.completion = json.dumps(self.equipment, ensure_ascii=False)

    def __repr__(self) -> str:
        return f"<{self.id} / Equipement: \n {json.dumps(self.equipment, indent=2, ensure_ascii=False)}\n>"

    def count_tokens(self, tokenizer):
        """Count tokens in the gear description"""
        return len(tokenizer.encode(self.prompt, add_special_tokens=False))
    @staticmethod
    def push_to_hub(dataset_name: str, train: list[Self], val: list[Self], test: list[Self]):
        """Push Item lists to HuggingFace Hub"""
        DatasetDict(
            {
                "train": Dataset.from_list([item.model_dump() for item in train]),
                "validation": Dataset.from_list([item.model_dump() for item in val]),
                "test": Dataset.from_list([item.model_dump() for item in test]),
            }
        ).push_to_hub(dataset_name)

    @classmethod
    def from_hub(cls, dataset_name: str) -> tuple[list[Self], list[Self], list[Self]]:
        """Load from HuggingFace Hub and reconstruct Items"""
        ds = load_dataset(dataset_name)
        return (
            [cls.model_validate(row) for row in ds["train"]],
            [cls.model_validate(row) for row in ds["validation"]],
            [cls.model_validate(row) for row in ds["test"]],
        )
    
    @staticmethod
    def parse(datapoint):
        return Equipment(
            id=datapoint["route_id"],
            gear=datapoint["gear"],
            equipment=datapoint["equipment"],
        )
