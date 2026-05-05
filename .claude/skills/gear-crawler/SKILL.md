---
name: gear-crawler
description: Extracts a gear list from a site and formats it as JSONL using a language model.
---

# gear-crawler
Here are the steps to run this tool:
1. Ensure that executable **generate_gear_dataset** exits in backend/cmd/generate_gear_dataset folder. You can build it by running `go build -o generate_gear_dataset backend/cmd/generate_gear_dataset/main.go` from the root of the repository.
2. User should provide 2 parameters: the output file path and the number of items to crawl. For example, `./generate_gear_dataset -out=gear_dataset.jsonl -limit=100` will generate a dataset of 100 gear items and save it to gear_dataset.jsonl.
3. If no parameters are provided, you should warn the user about the default values, which are 500 items and output file named gear_dataset.jsonl in the current directory.
4. Once generated, you should rework the jsonl file to match the expected format for the next steps in the pipeline. Each line in the jsonl file should be a JSON object with the following structure:
```json
{
  "route_id": "1898524", 
  "gear": "Sangles pour les relais\nCoinceurs peu utiles", 
  "equipment": [
    {"name": "Sangles", "quantity": 1, "notes": "obligatoire"}
  ]
}
```
You should generate the equipement list by using the script located in .claude/skills/gear-crawler/scripts/enrich_gear.py, which calls the OpenAI API to parse the gear text and extract the equipment information.