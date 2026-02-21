"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { authApi, APIToken } from "@/lib/api";

type ApiToken = APIToken;
import { DashboardLayout } from "@/components/dashboard-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Plus,
  Key,
  Trash2,
  Copy,
  CheckCircle2,
  Loader2,
  Shield,
  Clock,
  AlertTriangle,
} from "lucide-react";

const cardColors = [
  { gradient: "from-amber-500 to-orange-500", bg: "bg-amber-500/10", glow: "hover:shadow-amber-500/20" },
  { gradient: "from-violet-500 to-purple-500", bg: "bg-violet-500/10", glow: "hover:shadow-violet-500/20" },
  { gradient: "from-cyan-500 to-blue-500", bg: "bg-cyan-500/10", glow: "hover:shadow-cyan-500/20" },
  { gradient: "from-emerald-500 to-green-500", bg: "bg-emerald-500/10", glow: "hover:shadow-emerald-500/20" },
  { gradient: "from-pink-500 to-rose-500", bg: "bg-pink-500/10", glow: "hover:shadow-pink-500/20" },
];

function TokenCard({
  token,
  onRevoke,
  index,
}: {
  token: ApiToken;
  onRevoke: () => void;
  index: number;
}) {
  const color = cardColors[index % cardColors.length];

  return (
    <Card
      className={`group relative overflow-hidden border-border/30 bg-card/30 backdrop-blur-xl transition-all duration-500 hover:border-border/50 hover:bg-card/50 hover:shadow-xl ${color.glow}`}
      style={{ animationDelay: `${index * 0.1}s` }}
    >
      {/* Gradient accent */}
      <div className={`absolute left-0 top-0 h-full w-1.5 bg-gradient-to-b ${color.gradient}`} />

      <CardContent className="relative p-6 pl-8">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className={`flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br ${color.gradient} text-white shadow-lg`}>
              <Key className="h-6 w-6" />
            </div>
            <div className="space-y-1">
              <h3 className="text-lg font-semibold">{token.name}</h3>
              <p className="flex items-center gap-2 text-sm text-muted-foreground">
                <Clock className="h-3.5 w-3.5" />
                Created {new Date(token.created_at).toLocaleDateString()}
              </p>
              {token.last_used_at && (
                <p className="text-xs text-muted-foreground">
                  Last used {new Date(token.last_used_at).toLocaleDateString()}
                </p>
              )}
            </div>
          </div>

          <Button
            variant="ghost"
            size="sm"
            className="gap-2 text-red-500 hover:bg-red-500/10 hover:text-red-500"
            onClick={onRevoke}
          >
            <Trash2 className="h-4 w-4" />
            Revoke
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

export default function ApiKeysPage() {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [newTokenName, setNewTokenName] = useState("");
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const { data: tokens = [], isLoading } = useQuery({
    queryKey: ["tokens"],
    queryFn: authApi.listTokens,
  });

  const createMutation = useMutation({
    mutationFn: (name: string) => authApi.createToken(name),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["tokens"] });
      setCreatedToken(data.token);
      setNewTokenName("");
    },
  });

  const revokeMutation = useMutation({
    mutationFn: (id: string) => authApi.revokeToken(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["tokens"] });
    },
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    createMutation.mutate(newTokenName);
  };

  const handleCopy = async () => {
    if (createdToken) {
      await navigator.clipboard.writeText(createdToken);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleCloseCreate = () => {
    setIsCreateOpen(false);
    setCreatedToken(null);
    setNewTokenName("");
  };

  return (
    <DashboardLayout
      title="API Keys"
      description="Manage your API tokens for gateway access"
      actions={
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button className="gap-2 gradient-primary border-0 text-white glow-purple">
              <Plus className="h-4 w-4" />
              Create Token
            </Button>
          </DialogTrigger>
          <DialogContent className="border-border/30 bg-card/95 backdrop-blur-xl sm:max-w-md">
            {createdToken ? (
              <>
                <DialogHeader>
                  <DialogTitle className="flex items-center gap-2">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-emerald-500 to-green-500 text-white">
                      <CheckCircle2 className="h-4 w-4" />
                    </div>
                    Token Created
                  </DialogTitle>
                  <DialogDescription>
                    Make sure to copy your token now. You won&apos;t be able to see it again.
                  </DialogDescription>
                </DialogHeader>
                <div className="py-6">
                  <div className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-amber-500/10 to-orange-500/10 p-4 text-sm text-amber-500 border border-amber-500/20">
                    <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                    <span>This token will only be shown once</span>
                  </div>
                  <div className="mt-4 flex items-center gap-2">
                    <code className="flex-1 rounded-xl bg-muted/30 px-4 py-3 text-sm font-mono break-all border border-border/30">
                      {createdToken}
                    </code>
                    <Button
                      variant="outline"
                      size="icon"
                      onClick={handleCopy}
                      className={`flex-shrink-0 h-12 w-12 ${copied ? 'border-emerald-500/30 bg-emerald-500/10' : 'border-border/30'}`}
                    >
                      {copied ? (
                        <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                      ) : (
                        <Copy className="h-5 w-5" />
                      )}
                    </Button>
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={handleCloseCreate} className="w-full gradient-primary border-0 text-white">
                    Done
                  </Button>
                </DialogFooter>
              </>
            ) : (
              <form onSubmit={handleCreate}>
                <DialogHeader>
                  <DialogTitle className="flex items-center gap-2">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br from-amber-500 to-orange-500 text-white">
                      <Key className="h-4 w-4" />
                    </div>
                    Create API Token
                  </DialogTitle>
                  <DialogDescription>
                    Create a new token for API access
                  </DialogDescription>
                </DialogHeader>
                <div className="py-6">
                  <div className="space-y-2">
                    <Label htmlFor="name" className="text-sm font-medium">
                      Token Name
                    </Label>
                    <Input
                      id="name"
                      placeholder="e.g., Production API"
                      value={newTokenName}
                      onChange={(e) => setNewTokenName(e.target.value)}
                      required
                      className="h-11 bg-background/50 border-border/30"
                    />
                    <p className="text-xs text-muted-foreground">
                      Give your token a descriptive name for easy identification
                    </p>
                  </div>
                </div>
                <DialogFooter>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={handleCloseCreate}
                    className="border-border/30"
                  >
                    Cancel
                  </Button>
                  <Button type="submit" disabled={createMutation.isPending} className="gap-2 gradient-primary border-0 text-white">
                    {createMutation.isPending ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        Creating...
                      </>
                    ) : (
                      "Create Token"
                    )}
                  </Button>
                </DialogFooter>
              </form>
            )}
          </DialogContent>
        </Dialog>
      }
    >
      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Card
              key={i}
              className="h-28 animate-pulse border-border/30 bg-card/20"
            />
          ))}
        </div>
      ) : tokens.length === 0 ? (
        <Card className="border-dashed border-border/30 bg-card/20">
          <CardContent className="flex flex-col items-center justify-center py-20 text-center">
            <div className="relative mb-6">
              <div className="absolute inset-0 animate-pulse rounded-full bg-gradient-to-br from-amber-500/20 to-orange-500/20" />
              <div className="relative flex h-20 w-20 items-center justify-center rounded-full bg-gradient-to-br from-amber-500 to-orange-500 text-white shadow-lg">
                <Shield className="h-10 w-10" />
              </div>
            </div>
            <h3 className="mb-2 text-2xl font-bold">No API tokens</h3>
            <p className="mb-8 max-w-md text-muted-foreground">
              Create your first API token to start using the gateway programmatically
            </p>
            <Button onClick={() => setIsCreateOpen(true)} className="gap-2 gradient-primary border-0 text-white">
              <Plus className="h-4 w-4" />
              Create Your First Token
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <div className="flex items-center justify-between text-sm text-muted-foreground mb-2">
            <span>{tokens.filter((t: ApiToken) => t.name !== "Login" && t.name !== "Default").length} API Keys</span>
          </div>
          {tokens.filter((t: ApiToken) => t.name !== "Login" && t.name !== "Default").length === 0 ? (
            <Card className="border-dashed border-border/30 bg-card/20">
              <CardContent className="flex flex-col items-center justify-center py-12 text-center">
                <Key className="h-10 w-10 text-muted-foreground mb-4" />
                <h3 className="mb-2 text-lg font-semibold">No API tokens yet</h3>
                <p className="mb-4 text-sm text-muted-foreground">
                  Create an API token to access the gateway programmatically
                </p>
                <Button onClick={() => setIsCreateOpen(true)} size="sm" className="gap-2 gradient-primary border-0 text-white">
                  <Plus className="h-4 w-4" />
                  Create Token
                </Button>
              </CardContent>
            </Card>
          ) : (
            tokens.filter((t: ApiToken) => t.name !== "Login" && t.name !== "Default").map((token: ApiToken, index: number) => (
              <TokenCard
                key={token.id}
                token={token}
                index={index}
                onRevoke={() => revokeMutation.mutate(token.id)}
              />
            ))
          )}
        </div>
      )}
    </DashboardLayout>
  );
}
