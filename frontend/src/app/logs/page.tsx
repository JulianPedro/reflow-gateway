"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { logsApi, RequestLog } from "@/lib/api";
import { DashboardLayout } from "@/components/dashboard-layout";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  RefreshCw,
  CheckCircle2,
  XCircle,
  Clock,
  ChevronLeft,
  ChevronRight,
  Activity,
  ScrollText,
} from "lucide-react";

function StatusBadge({ status }: { status: number }) {
  const isSuccess = status >= 200 && status < 300;

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-semibold ${
        isSuccess
          ? "bg-gradient-to-r from-emerald-500/20 to-green-500/20 text-emerald-500"
          : "bg-gradient-to-r from-red-500/20 to-rose-500/20 text-red-500"
      }`}
    >
      {isSuccess ? (
        <CheckCircle2 className="h-3.5 w-3.5" />
      ) : (
        <XCircle className="h-3.5 w-3.5" />
      )}
      {status}
    </span>
  );
}

const methodColors: Record<string, string> = {
  "initialize": "from-violet-500 to-purple-500",
  "tools/list": "from-cyan-500 to-blue-500",
  "tools/call": "from-emerald-500 to-green-500",
  "resources/list": "from-amber-500 to-orange-500",
  "resources/read": "from-pink-500 to-rose-500",
  "prompts/list": "from-indigo-500 to-violet-500",
  "prompts/get": "from-teal-500 to-cyan-500",
};

function getMethodColor(method: string): string {
  return methodColors[method] || "from-slate-500 to-gray-500";
}

export default function LogsPage() {
  const [page, setPage] = useState(0);
  const limit = 20;

  const {
    data: logs = [],
    isLoading,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ["logs", page],
    queryFn: () => logsApi.list(limit, page * limit),
  });

  return (
    <DashboardLayout
      title="Request Logs"
      description="Monitor all MCP requests processed by the gateway"
      actions={
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetch()}
            disabled={isFetching}
            className="gap-2 border-border/30 hover:bg-emerald-500/10 hover:text-emerald-500 hover:border-emerald-500/30"
          >
            <RefreshCw
              className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
        </div>
      }
    >
      <Card className="overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl">
        <CardContent className="p-0">
          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-20">
              <div className="relative">
                <div className="absolute inset-0 animate-pulse rounded-full bg-gradient-to-br from-emerald-500/20 to-cyan-500/20" />
                <div className="relative flex h-16 w-16 items-center justify-center rounded-full bg-gradient-to-br from-emerald-500 to-green-500 text-white">
                  <RefreshCw className="h-8 w-8 animate-spin" />
                </div>
              </div>
              <p className="mt-4 text-sm text-muted-foreground">Loading logs...</p>
            </div>
          ) : logs.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <div className="relative mb-6">
                <div className="absolute inset-0 animate-pulse rounded-full bg-gradient-to-br from-emerald-500/20 to-cyan-500/20" />
                <div className="relative flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-emerald-500 to-green-500 text-white shadow-lg">
                  <Activity className="h-10 w-10" />
                </div>
              </div>
              <h3 className="mb-2 text-xl font-semibold">No requests logged yet</h3>
              <p className="max-w-md text-muted-foreground">
                Requests will appear here once you start using the gateway
              </p>
            </div>
          ) : (
            <>
              {/* Table */}
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-border/30 bg-gradient-to-r from-muted/20 to-muted/10">
                      <th className="px-6 py-4 text-left text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                        Time
                      </th>
                      <th className="px-6 py-4 text-left text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                        Method
                      </th>
                      <th className="px-6 py-4 text-left text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                        Target
                      </th>
                      <th className="px-6 py-4 text-left text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                        Status
                      </th>
                      <th className="px-6 py-4 text-left text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                        Duration
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border/30">
                    {logs.map((log: RequestLog, index: number) => (
                      <tr
                        key={log.id}
                        className="group transition-colors hover:bg-gradient-to-r hover:from-violet-500/5 hover:to-cyan-500/5"
                        style={{ animationDelay: `${index * 0.02}s` }}
                      >
                        <td className="whitespace-nowrap px-6 py-4">
                          <div className="flex flex-col">
                            <span className="text-sm font-medium">
                              {new Date(log.created_at).toLocaleTimeString()}
                            </span>
                            <span className="text-xs text-muted-foreground">
                              {new Date(log.created_at).toLocaleDateString()}
                            </span>
                          </div>
                        </td>
                        <td className="whitespace-nowrap px-6 py-4">
                          <code className={`rounded-lg bg-gradient-to-r ${getMethodColor(log.method)} px-3 py-1.5 text-sm font-semibold text-white`}>
                            {log.method}
                          </code>
                        </td>
                        <td className="whitespace-nowrap px-6 py-4">
                          <span className="text-sm text-muted-foreground">
                            {log.target_name || (
                              <span className="italic opacity-50">No target</span>
                            )}
                          </span>
                        </td>
                        <td className="whitespace-nowrap px-6 py-4">
                          <StatusBadge status={log.response_status} />
                        </td>
                        <td className="whitespace-nowrap px-6 py-4">
                          <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
                            <Clock className="h-3.5 w-3.5" />
                            <span className="tabular-nums font-medium">
                              {log.duration_ms}ms
                            </span>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="flex items-center justify-between border-t border-border/30 bg-gradient-to-r from-muted/10 to-muted/5 px-6 py-4">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((p) => Math.max(0, p - 1))}
                  disabled={page === 0}
                  className="gap-2 border-border/30 hover:bg-violet-500/10 hover:text-violet-500 hover:border-violet-500/30"
                >
                  <ChevronLeft className="h-4 w-4" />
                  Previous
                </Button>
                <div className="flex items-center gap-2">
                  <span className="rounded-lg bg-gradient-to-r from-violet-500/20 to-purple-500/20 px-4 py-1.5 text-sm font-medium">
                    Page {page + 1}
                  </span>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setPage((p) => p + 1)}
                  disabled={logs.length < limit}
                  className="gap-2 border-border/30 hover:bg-cyan-500/10 hover:text-cyan-500 hover:border-cyan-500/30"
                >
                  Next
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </DashboardLayout>
  );
}
