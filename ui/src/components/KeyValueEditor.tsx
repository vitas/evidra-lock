interface KeyValuePair {
  key: string;
  value: string;
}

interface KeyValueEditorProps {
  pairs: KeyValuePair[];
  onChange: (pairs: KeyValuePair[]) => void;
}

export function KeyValueEditor({ pairs, onChange }: KeyValueEditorProps) {
  const updatePair = (index: number, field: "key" | "value", val: string) => {
    const next = pairs.map((p, i) =>
      i === index ? { ...p, [field]: val } : p,
    );
    onChange(next);
  };

  const removePair = (index: number) => {
    onChange(pairs.filter((_, i) => i !== index));
  };

  const addPair = () => {
    onChange([...pairs, { key: "", value: "" }]);
  };

  return (
    <div className="kv-editor">
      {pairs.map((pair, i) => (
        <div className="kv-row" key={i}>
          <input
            type="text"
            placeholder="key"
            value={pair.key}
            onChange={(e) => updatePair(i, "key", e.target.value)}
          />
          <input
            type="text"
            placeholder="value"
            value={pair.value}
            onChange={(e) => updatePair(i, "value", e.target.value)}
          />
          <button type="button" onClick={() => removePair(i)} className="kv-remove">
            &times;
          </button>
        </div>
      ))}
      <button type="button" onClick={addPair} className="kv-add">
        + Add param
      </button>
    </div>
  );
}
