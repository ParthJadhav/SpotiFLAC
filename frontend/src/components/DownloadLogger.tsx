import { useState, useEffect, useRef } from "react";
import { Trash2, X, Copy, Check, Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";

interface LogEntry {
  timestamp: Date;
  level: 'info' | 'success' | 'warning' | 'error' | 'debug';
  message: string;
}

const levelColors: Record<string, string> = {
  info: "text-blue-500",
  success: "text-green-500",
  warning: "text-yellow-500",
  error: "text-red-500",
  debug: "text-gray-500",
};

function formatTime(date: Date): string {
  return date.toLocaleTimeString("en-US", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

export function DownloadLogger() {
  const [open, setOpen] = useState(false);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [copied, setCopied] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  const addLog = (level: LogEntry['level'], message: string) => {
    const newLog: LogEntry = {
      timestamp: new Date(),
      level,
      message,
    };
    setLogs(prev => [...prev, newLog]);
  };

  // Listen for download events
  useEffect(() => {
    const handleDownloadStart = (event: CustomEvent) => {
      const { trackName, artistName, service } = event.detail;
      addLog('info', `🎵 Starting download: ${trackName} - ${artistName} [${service}]`);
    };

    const handleDownloadProgress = (event: CustomEvent) => {
      const { message } = event.detail;
      addLog('debug', `⏳ ${message}`);
    };

    const handleDownloadSuccess = (event: CustomEvent) => {
      const { trackName, artistName, filename } = event.detail;
      addLog('success', `✅ Downloaded: ${trackName} - ${artistName} → ${filename}`);
    };

    const handleDownloadError = (event: CustomEvent) => {
      const { trackName, artistName, error } = event.detail;
      addLog('error', `❌ Failed: ${trackName} - ${artistName} - ${error}`);
    };

    const handleDownloadSkipped = (event: CustomEvent) => {
      const { trackName, artistName, reason } = event.detail;
      addLog('warning', `⏭️ Skipped: ${trackName} - ${artistName} - ${reason}`);
    };

    window.addEventListener('download-start', handleDownloadStart as EventListener);
    window.addEventListener('download-progress', handleDownloadProgress as EventListener);
    window.addEventListener('download-success', handleDownloadSuccess as EventListener);
    window.addEventListener('download-error', handleDownloadError as EventListener);
    window.addEventListener('download-skipped', handleDownloadSkipped as EventListener);

    return () => {
      window.removeEventListener('download-start', handleDownloadStart as EventListener);
      window.removeEventListener('download-progress', handleDownloadProgress as EventListener);
      window.removeEventListener('download-success', handleDownloadSuccess as EventListener);
      window.removeEventListener('download-error', handleDownloadError as EventListener);
      window.removeEventListener('download-skipped', handleDownloadSkipped as EventListener);
    };
  }, []);

  const handleClear = () => {
    setLogs([]);
  };

  const handleCopy = async () => {
    const logText = logs
      .map((log) => `[${formatTime(log.timestamp)}] [${log.level}] ${log.message}`)
      .join("\\n");
    
    try {
      await navigator.clipboard.writeText(logText);
      setCopied(true);
      setTimeout(() => setCopied(false), 500);
    } catch (err) {
      console.error("Failed to copy logs:", err);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="h-6 w-6 opacity-50 hover:opacity-100"
          title="Download Logs"
        >
          <Download className="h-3.5 w-3.5" />
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[700px] max-h-[80vh] p-6 [&>button]:hidden">
        <DialogTitle className="text-sm font-medium">Download Logs</DialogTitle>
        <div className="absolute right-4 top-4 flex items-center gap-1">
          <span className="text-xs text-muted-foreground mr-2">
            {logs.length} entries
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 opacity-70 hover:opacity-100"
            onClick={handleCopy}
            disabled={logs.length === 0}
            title="Copy logs"
          >
            {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 opacity-70 hover:opacity-100"
            onClick={handleClear}
            title="Clear logs"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-6 w-6 opacity-70 hover:opacity-100"
            onClick={() => setOpen(false)}
            title="Close"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div
          ref={scrollRef}
          className="h-[450px] overflow-y-auto bg-muted/50 rounded-md p-3 font-mono text-xs"
        >
          {logs.length === 0 ? (
            <p className="text-muted-foreground lowercase">no download logs yet...</p>
          ) : (
            logs.map((log, i) => (
              <div key={i} className="flex gap-2 py-0.5 hover:bg-muted/30 rounded px-1">
                <span className="text-muted-foreground shrink-0">
                  [{formatTime(log.timestamp)}]
                </span>
                <span className={`shrink-0 w-16 ${levelColors[log.level]}`}>
                  [{log.level}]
                </span>
                <span className="break-all">{log.message}</span>
              </div>
            ))
          )}
        </div>
        {logs.length > 0 && (
          <div className="flex justify-between items-center text-xs text-muted-foreground pt-2 border-t">
            <span>
              Success: {logs.filter(l => l.level === 'success').length} | 
              Errors: {logs.filter(l => l.level === 'error').length} | 
              Skipped: {logs.filter(l => l.level === 'warning').length}
            </span>
            <span>Real-time download monitoring</span>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}