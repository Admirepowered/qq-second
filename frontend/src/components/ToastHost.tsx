// toast 渲染层（挂在 App 根部）。

import { useToast } from '../store/toast';

export function ToastHost() {
  const { message, kind } = useToast();
  if (!message) return null;
  return (
    <div className={`toast-host${kind === 'error' ? ' error' : ''}`} role="status">
      {message}
    </div>
  );
}
