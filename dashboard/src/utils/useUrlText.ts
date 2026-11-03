import { useEffect, useRef, useState } from "react";

export function useUrlText(urlValue: string, commit: (value: string) => void, delay = 250) {
  const [text, setText] = useState(urlValue);
  const pushed = useRef(urlValue);
  const latest = useRef(commit);
  latest.current = commit;

  useEffect(() => {
    if (urlValue !== pushed.current) {
      pushed.current = urlValue;
      setText(urlValue);
    }
  }, [urlValue]);

  useEffect(() => {
    if (text === pushed.current) return;
    const timer = setTimeout(() => {
      pushed.current = text;
      latest.current(text);
    }, delay);
    return () => clearTimeout(timer);
  }, [text, delay]);

  return [text, setText] as const;
}
