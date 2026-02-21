"use client";

import { useQuery } from "@tanstack/react-query";
import { targetsApi, policiesApi, healthApi, Target, AuthorizationPolicy } from "@/lib/api";
import { DashboardLayout } from "@/components/dashboard-layout";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import Link from "next/link";
import {
  Server,
  Zap,
  ArrowUpRight,
  Heart,
  Plus,
  ShieldCheck,
  Key,
} from "lucide-react";

function StatCard({
  title,
  value,
  subtitle,
  icon: Icon,
  color,
}: {
  title: string;
  value: string | number;
  subtitle: string;
  icon: React.ElementType;
  color: string;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">
          {title}
        </CardTitle>
        <div className={`flex h-11 w-11 items-center justify-center rounded-lg ${color} text-white`}>
          <Icon className="h-5 w-5" />
        </div>
      </CardHeader>
      <CardContent>
        <div className="text-3xl font-bold">{value}</div>
        <p className="mt-1 text-sm text-muted-foreground">{subtitle}</p>
      </CardContent>
    </Card>
  );
}

function TargetCard({ target }: { target: Target }) {
  return (
    <div className="flex items-center justify-between rounded-lg border border-border/50 p-4">
      <div className="flex items-center gap-3">
        <div className={`flex h-11 w-11 items-center justify-center rounded-lg ${target.enabled ? "bg-cyan-500 text-white" : "bg-muted text-muted-foreground"}`}>
          <Server className="h-5 w-5" />
        </div>
        <div>
          <h3 className="font-medium">{target.name}</h3>
          <p className="text-sm text-muted-foreground">{target.url}</p>
        </div>
      </div>
      <span
        className={`rounded-full px-2.5 py-1 text-xs font-medium ${
          target.enabled
            ? "bg-emerald-500/10 text-emerald-500"
            : "bg-muted text-muted-foreground"
        }`}
      >
        {target.enabled ? "Active" : "Disabled"}
      </span>
    </div>
  );
}

export default function DashboardPage() {
  const { data: targets = [] } = useQuery({
    queryKey: ["targets"],
    queryFn: targetsApi.list,
  });

  const { data: policies = [] } = useQuery({
    queryKey: ["policies"],
    queryFn: policiesApi.list,
  });

  const { data: health, isError: healthError } = useQuery({
    queryKey: ["health"],
    queryFn: healthApi.check,
    refetchInterval: 30000,
  });

  const enabledTargets = targets.filter((t: Target) => t.enabled);
  const enabledPolicies = policies.filter((p: AuthorizationPolicy) => p.enabled);

  const isHealthy = health?.status === "ok" && !healthError;

  return (
    <DashboardLayout
      title="Dashboard"
      description="Overview of your MCP Gateway"
    >
      {/* Stats */}
      <div className="mb-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total MCP Servers"
          value={targets.length}
          subtitle={`${enabledTargets.length} currently active`}
          icon={Server}
          color="bg-violet-500"
        />
        <StatCard
          title="Active MCP Servers"
          value={enabledTargets.length}
          subtitle="Enabled and available"
          icon={Zap}
          color="bg-cyan-500"
        />
        <StatCard
          title="Gateway Health"
          value={isHealthy ? "Healthy" : "Unreachable"}
          subtitle={isHealthy ? "All systems operational" : "Cannot reach backend"}
          icon={Heart}
          color={isHealthy ? "bg-emerald-500" : "bg-red-500"}
        />
        <StatCard
          title="Policies Active"
          value={enabledPolicies.length}
          subtitle={`${policies.length} total policies`}
          icon={ShieldCheck}
          color="bg-amber-500"
        />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Targets */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle className="text-lg">MCP Targets</CardTitle>
              <p className="mt-1 text-sm text-muted-foreground">
                Connected upstream servers
              </p>
            </div>
            <Link href="/targets">
              <Button variant="outline" size="sm" className="gap-1">
                Manage
                <ArrowUpRight className="h-3.5 w-3.5" />
              </Button>
            </Link>
          </CardHeader>
          <CardContent>
            {targets.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Server className="mb-3 h-10 w-10 text-muted-foreground" />
                <h3 className="mb-1 font-medium">No targets configured</h3>
                <p className="mb-4 text-sm text-muted-foreground">
                  Add your first MCP server to get started
                </p>
                <Link href="/targets">
                  <Button size="sm" className="gap-1">
                    Add Target
                    <ArrowUpRight className="h-3.5 w-3.5" />
                  </Button>
                </Link>
              </div>
            ) : (
              <div className="space-y-3">
                {targets.slice(0, 5).map((target: Target) => (
                  <TargetCard key={target.id} target={target} />
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Quick Actions */}
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Quick Actions</CardTitle>
            <p className="mt-1 text-sm text-muted-foreground">
              Common tasks and shortcuts
            </p>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <Link href="/targets" className="block">
                <button className="flex w-full items-center gap-3 rounded-lg border border-border/50 p-4 text-left transition-colors hover:bg-muted/50">
                  <div className="flex h-11 w-11 items-center justify-center rounded-lg bg-violet-500 text-white">
                    <Plus className="h-5 w-5" />
                  </div>
                  <span className="font-medium">Add MCP Server</span>
                  <ArrowUpRight className="ml-auto h-4 w-4 text-muted-foreground" />
                </button>
              </Link>
              <Link href="/policies" className="block">
                <button className="flex w-full items-center gap-3 rounded-lg border border-border/50 p-4 text-left transition-colors hover:bg-muted/50">
                  <div className="flex h-11 w-11 items-center justify-center rounded-lg bg-emerald-500 text-white">
                    <ShieldCheck className="h-5 w-5" />
                  </div>
                  <span className="font-medium">Create Policy</span>
                  <ArrowUpRight className="ml-auto h-4 w-4 text-muted-foreground" />
                </button>
              </Link>
              <Link href="/api-keys" className="block">
                <button className="flex w-full items-center gap-3 rounded-lg border border-border/50 p-4 text-left transition-colors hover:bg-muted/50">
                  <div className="flex h-11 w-11 items-center justify-center rounded-lg bg-amber-500 text-white">
                    <Key className="h-5 w-5" />
                  </div>
                  <span className="font-medium">Generate API Key</span>
                  <ArrowUpRight className="ml-auto h-4 w-4 text-muted-foreground" />
                </button>
              </Link>
            </div>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  );
}
