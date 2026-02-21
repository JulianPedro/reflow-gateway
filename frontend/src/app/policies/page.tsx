"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  policiesApi,
  targetsApi,
  AuthorizationPolicy,
  CreatePolicyRequest,
  Target,
} from "@/lib/api";
import { DashboardLayout } from "@/components/dashboard-layout";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
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
  Trash2,
  ShieldCheck,
  ShieldX,
  MoreVertical,
  Loader2,
  UserCircle,
  Users,
  Crown,
  Globe,
  Pencil,
  UserPlus,
} from "lucide-react";

const subjectTypeIcons = {
  user: UserCircle,
  role: Crown,
  group: Users,
  everyone: Globe,
};

function PolicyCard({
  policy,
  targets,
  onToggle,
  onDelete,
  onEdit,
  onAddSubject,
  onDeleteSubject,
}: {
  policy: AuthorizationPolicy;
  targets: Target[];
  onToggle: (enabled: boolean) => void;
  onDelete: () => void;
  onEdit: () => void;
  onAddSubject: () => void;
  onDeleteSubject: (subjectId: string) => void;
}) {
  const [showMenu, setShowMenu] = useState(false);
  const targetName = policy.target_id
    ? targets.find((t) => t.id === policy.target_id)?.name || "Unknown"
    : "All Targets";

  return (
    <Card>
      <CardContent className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex items-start gap-4">
            <div className={`flex h-11 w-11 items-center justify-center rounded-lg ${policy.enabled
                ? policy.effect === "allow"
                  ? "bg-emerald-500 text-white"
                  : "bg-red-500 text-white"
                : "bg-muted text-muted-foreground"
              }`}>
              {policy.effect === "allow" ? (
                <ShieldCheck className="h-5 w-5" />
              ) : (
                <ShieldX className="h-5 w-5" />
              )}
            </div>
            <div className="space-y-2">
              <h3 className="font-semibold">{policy.name}</h3>
              {policy.description && (
                <p className="text-sm text-muted-foreground">{policy.description}</p>
              )}
              <div className="flex flex-wrap gap-2">
                <span className={`inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium ${policy.effect === "allow"
                    ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                    : "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                  }`}>
                  {policy.effect === "allow" ? <ShieldCheck className="h-3 w-3" /> : <ShieldX className="h-3 w-3" />}
                  {policy.effect.toUpperCase()}
                </span>
                <span className="rounded bg-muted px-2 py-0.5 text-xs">{targetName}</span>
                <span className="rounded bg-muted px-2 py-0.5 text-xs">
                  {policy.resource_type === "all" ? "All Resources" : policy.resource_type}
                </span>
                <span className="rounded bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400 px-2 py-0.5 text-xs">
                  Priority: {policy.priority}
                </span>
              </div>

              {policy.subjects && policy.subjects.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1">
                  {policy.subjects.map((subject) => {
                    const Icon = subjectTypeIcons[subject.subject_type];
                    return (
                      <span
                        key={subject.id}
                        className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-1 text-xs"
                      >
                        <Icon className="h-3 w-3" />
                        {subject.subject_type === "everyone"
                          ? "Everyone"
                          : subject.subject_value || subject.subject_type}
                        <button
                          onClick={() => onDeleteSubject(subject.id)}
                          className="ml-0.5 rounded p-0.5 hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900/30"
                        >
                          <Trash2 className="h-3 w-3" />
                        </button>
                      </span>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              <span className={`text-sm ${policy.enabled ? "text-emerald-600" : "text-muted-foreground"}`}>
                {policy.enabled ? "Active" : "Disabled"}
              </span>
              <Switch checked={policy.enabled} onCheckedChange={onToggle} />
            </div>

            <div className="relative">
              <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => setShowMenu(!showMenu)}>
                <MoreVertical className="h-4 w-4" />
              </Button>

              {showMenu && (
                <>
                  <div className="fixed inset-0 z-10" onClick={() => setShowMenu(false)} />
                  <div className="absolute right-0 top-full z-20 mt-1 w-44 rounded-lg border bg-popover p-1 shadow-lg">
                    <button
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted"
                      onClick={() => { onEdit(); setShowMenu(false); }}
                    >
                      <Pencil className="h-4 w-4" />
                      Edit Policy
                    </button>
                    <button
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted"
                      onClick={() => { onAddSubject(); setShowMenu(false); }}
                    >
                      <UserPlus className="h-4 w-4" />
                      Add Subject
                    </button>
                    <hr className="my-1 border-border" />
                    <button
                      className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-red-600 hover:bg-red-50 dark:hover:bg-red-950"
                      onClick={() => { onDelete(); setShowMenu(false); }}
                    >
                      <Trash2 className="h-4 w-4" />
                      Delete Policy
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export default function PoliciesPage() {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isEditOpen, setIsEditOpen] = useState(false);
  const [isSubjectOpen, setIsSubjectOpen] = useState(false);
  const [selectedPolicy, setSelectedPolicy] = useState<AuthorizationPolicy | null>(null);
  const [newPolicy, setNewPolicy] = useState<CreatePolicyRequest>({
    name: "", description: "", target_id: undefined, resource_type: "all",
    resource_pattern: "", effect: "allow", priority: 0, enabled: true,
  });
  const [newPolicySubjects, setNewPolicySubjects] = useState<{ subject_type: "user" | "role" | "group" | "everyone"; subject_value: string }[]>([]);
  const [newSubject, setNewSubject] = useState({
    subject_type: "everyone" as "user" | "role" | "group" | "everyone",
    subject_value: "",
  });

  const { data: policies = [], isLoading } = useQuery({
    queryKey: ["policies"],
    queryFn: policiesApi.list,
  });

  const { data: targets = [] } = useQuery({
    queryKey: ["targets"],
    queryFn: targetsApi.list,
  });

  const createMutation = useMutation({
    mutationFn: (data: CreatePolicyRequest) => policiesApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["policies"] });
      setIsCreateOpen(false);
      setNewPolicy({
        name: "", description: "", target_id: undefined, resource_type: "all",
        resource_pattern: "", effect: "allow", priority: 0, enabled: true,
      });
      setNewPolicySubjects([]);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<CreatePolicyRequest> }) =>
      policiesApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["policies"] });
      setIsEditOpen(false);
      setSelectedPolicy(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => policiesApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["policies"] }),
  });

  const addSubjectMutation = useMutation({
    mutationFn: ({ policyId, data }: { policyId: string; data: { subject_type: string; subject_value?: string } }) =>
      policiesApi.addSubject(policyId, data as any),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["policies"] });
      setIsSubjectOpen(false);
      setSelectedPolicy(null);
      setNewSubject({ subject_type: "everyone", subject_value: "" });
    },
  });

  const deleteSubjectMutation = useMutation({
    mutationFn: ({ policyId, subjectId }: { policyId: string; subjectId: string }) =>
      policiesApi.deleteSubject(policyId, subjectId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["policies"] }),
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    const data: CreatePolicyRequest = { ...newPolicy };
    if (!data.target_id) delete data.target_id;
    if (!data.resource_pattern) delete data.resource_pattern;
    // Add subjects to the request
    if (newPolicySubjects.length > 0) {
      data.subjects = newPolicySubjects.map(s => ({
        subject_type: s.subject_type,
        subject_value: s.subject_type === "everyone" ? undefined : s.subject_value,
      }));
    }
    createMutation.mutate(data);
  };

  const handleUpdate = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedPolicy) {
      updateMutation.mutate({
        id: selectedPolicy.id,
        data: {
          name: selectedPolicy.name,
          description: selectedPolicy.description,
          target_id: selectedPolicy.target_id,
          resource_type: selectedPolicy.resource_type,
          resource_pattern: selectedPolicy.resource_pattern,
          effect: selectedPolicy.effect,
          priority: selectedPolicy.priority,
          enabled: selectedPolicy.enabled,
        },
      });
    }
  };

  const handleAddSubject = (e: React.FormEvent) => {
    e.preventDefault();
    if (selectedPolicy) {
      addSubjectMutation.mutate({
        policyId: selectedPolicy.id,
        data: {
          subject_type: newSubject.subject_type,
          subject_value: newSubject.subject_type === "everyone" ? undefined : newSubject.subject_value,
        },
      });
    }
  };

  return (
    <DashboardLayout
      title="Authorization Policies"
      description="Manage access control policies for MCP resources"
      actions={
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="h-4 w-4 mr-2" />
              Add Policy
            </Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg max-h-[85vh] overflow-y-auto">
            <form onSubmit={handleCreate}>
              <DialogHeader>
                <DialogTitle>Create Policy</DialogTitle>
                <DialogDescription>Define access rules for MCP resources</DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="name">Name</Label>
                  <Input
                    id="name"
                    placeholder="e.g., Allow Admin Access"
                    value={newPolicy.name}
                    onChange={(e) => setNewPolicy({ ...newPolicy, name: e.target.value })}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="description">Description</Label>
                  <Input
                    id="description"
                    placeholder="Optional description"
                    value={newPolicy.description}
                    onChange={(e) => setNewPolicy({ ...newPolicy, description: e.target.value })}
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="effect">Effect</Label>
                    <select
                      id="effect"
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={newPolicy.effect}
                      onChange={(e) => setNewPolicy({ ...newPolicy, effect: e.target.value as "allow" | "deny" })}
                    >
                      <option value="allow">Allow</option>
                      <option value="deny">Deny</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="priority">Priority</Label>
                    <Input
                      id="priority"
                      type="number"
                      value={newPolicy.priority}
                      onChange={(e) => setNewPolicy({ ...newPolicy, priority: parseInt(e.target.value) || 0 })}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="target">Target</Label>
                  <select
                    id="target"
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={newPolicy.target_id || ""}
                    onChange={(e) => setNewPolicy({ ...newPolicy, target_id: e.target.value || undefined })}
                  >
                    <option value="">All Targets</option>
                    {targets.map((target) => (
                      <option key={target.id} value={target.id}>{target.name}</option>
                    ))}
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="resource_type">Resource Type</Label>
                    <select
                      id="resource_type"
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={newPolicy.resource_type}
                      onChange={(e) => setNewPolicy({ ...newPolicy, resource_type: e.target.value })}
                    >
                      <option value="all">All</option>
                      <option value="tool">Tools</option>
                      <option value="resource">Resources</option>
                      <option value="prompt">Prompts</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="resource_pattern">Pattern (Regex)</Label>
                    <Input
                      id="resource_pattern"
                      placeholder="e.g., ^file_.*"
                      value={newPolicy.resource_pattern}
                      onChange={(e) => setNewPolicy({ ...newPolicy, resource_pattern: e.target.value })}
                    />
                  </div>
                </div>

                {/* Subjects Section */}
                <div className="border-t border-border pt-4">
                  <div className="flex items-center justify-between mb-3">
                    <Label className="flex items-center gap-2">
                      <Users className="h-4 w-4" />
                      Subjects (Who this applies to)
                    </Label>
                  </div>

                  {/* Add subject form */}
                  <div className="flex gap-2 mb-3">
                    <select
                      className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={newSubject.subject_type}
                      onChange={(e) => setNewSubject({ ...newSubject, subject_type: e.target.value as any, subject_value: e.target.value === "everyone" ? "" : newSubject.subject_value })}
                    >
                      <option value="everyone">Everyone</option>
                      <option value="role">Role</option>
                      <option value="group">Group</option>
                      <option value="user">User (ID)</option>
                    </select>
                    {newSubject.subject_type !== "everyone" && (
                      <Input
                        className="flex-1"
                        placeholder={newSubject.subject_type === "role" ? "e.g., admin" : newSubject.subject_type === "group" ? "e.g., developers" : "User UUID"}
                        value={newSubject.subject_value}
                        onChange={(e) => setNewSubject({ ...newSubject, subject_value: e.target.value })}
                      />
                    )}
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        if (newSubject.subject_type === "everyone" || newSubject.subject_value) {
                          setNewPolicySubjects([...newPolicySubjects, { ...newSubject }]);
                          setNewSubject({ subject_type: "everyone", subject_value: "" });
                        }
                      }}
                    >
                      <Plus className="h-4 w-4" />
                    </Button>
                  </div>

                  {/* Subject list */}
                  {newPolicySubjects.length === 0 ? (
                    <p className="text-sm text-muted-foreground text-center py-2">
                      No subjects added. Policy will apply to no one until subjects are added.
                    </p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {newPolicySubjects.map((subject, index) => {
                        const Icon = subjectTypeIcons[subject.subject_type];
                        return (
                          <span
                            key={index}
                            className="inline-flex items-center gap-1.5 rounded-md bg-muted px-2.5 py-1.5 text-sm"
                          >
                            <Icon className="h-3.5 w-3.5" />
                            {subject.subject_type === "everyone" ? "Everyone" : subject.subject_value}
                            <button
                              type="button"
                              onClick={() => setNewPolicySubjects(newPolicySubjects.filter((_, i) => i !== index))}
                              className="ml-1 rounded p-0.5 hover:bg-red-100 hover:text-red-600 dark:hover:bg-red-900/30"
                            >
                              <Trash2 className="h-3 w-3" />
                            </button>
                          </span>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setIsCreateOpen(false)}>Cancel</Button>
                <Button type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      }
    >
      {isLoading ? (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => <Card key={i} className="h-28 animate-pulse bg-muted/50" />)}
        </div>
      ) : policies.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16 text-center">
            <ShieldCheck className="h-12 w-12 text-muted-foreground mb-4" />
            <h3 className="text-lg font-semibold mb-2">No policies configured</h3>
            <p className="text-muted-foreground mb-6">
              Create authorization policies to control access to MCP resources
            </p>
            <Button onClick={() => setIsCreateOpen(true)}>
              <Plus className="h-4 w-4 mr-2" />
              Create Policy
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {policies.map((policy) => (
            <PolicyCard
              key={policy.id}
              policy={policy}
              targets={targets}
              onToggle={(enabled) => updateMutation.mutate({ id: policy.id, data: { enabled } })}
              onDelete={() => deleteMutation.mutate(policy.id)}
              onEdit={() => { setSelectedPolicy(policy); setIsEditOpen(true); }}
              onAddSubject={() => { setSelectedPolicy(policy); setIsSubjectOpen(true); }}
              onDeleteSubject={(subjectId) => deleteSubjectMutation.mutate({ policyId: policy.id, subjectId })}
            />
          ))}
        </div>
      )}

      {/* Edit Policy Dialog */}
      <Dialog open={isEditOpen} onOpenChange={setIsEditOpen}>
        <DialogContent className="max-w-lg">
          <form onSubmit={handleUpdate}>
            <DialogHeader>
              <DialogTitle>Edit Policy</DialogTitle>
            </DialogHeader>
            {selectedPolicy && (
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="edit-name">Name</Label>
                  <Input
                    id="edit-name"
                    value={selectedPolicy.name}
                    onChange={(e) => setSelectedPolicy({ ...selectedPolicy, name: e.target.value })}
                    required
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-description">Description</Label>
                  <Input
                    id="edit-description"
                    value={selectedPolicy.description || ""}
                    onChange={(e) => setSelectedPolicy({ ...selectedPolicy, description: e.target.value })}
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="edit-effect">Effect</Label>
                    <select
                      id="edit-effect"
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={selectedPolicy.effect}
                      onChange={(e) => setSelectedPolicy({ ...selectedPolicy, effect: e.target.value as "allow" | "deny" })}
                    >
                      <option value="allow">Allow</option>
                      <option value="deny">Deny</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit-priority">Priority</Label>
                    <Input
                      id="edit-priority"
                      type="number"
                      value={selectedPolicy.priority}
                      onChange={(e) => setSelectedPolicy({ ...selectedPolicy, priority: parseInt(e.target.value) || 0 })}
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="edit-target">Target</Label>
                  <select
                    id="edit-target"
                    className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                    value={selectedPolicy.target_id || ""}
                    onChange={(e) => setSelectedPolicy({ ...selectedPolicy, target_id: e.target.value || undefined })}
                  >
                    <option value="">All Targets</option>
                    {targets.map((target) => (
                      <option key={target.id} value={target.id}>{target.name}</option>
                    ))}
                  </select>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="edit-resource_type">Resource Type</Label>
                    <select
                      id="edit-resource_type"
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                      value={selectedPolicy.resource_type}
                      onChange={(e) => setSelectedPolicy({ ...selectedPolicy, resource_type: e.target.value })}
                    >
                      <option value="all">All</option>
                      <option value="tool">Tools</option>
                      <option value="resource">Resources</option>
                      <option value="prompt">Prompts</option>
                    </select>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="edit-resource_pattern">Pattern</Label>
                    <Input
                      id="edit-resource_pattern"
                      value={selectedPolicy.resource_pattern || ""}
                      onChange={(e) => setSelectedPolicy({ ...selectedPolicy, resource_pattern: e.target.value })}
                    />
                  </div>
                </div>
              </div>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsEditOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={updateMutation.isPending}>
                {updateMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Save
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Add Subject Dialog */}
      <Dialog open={isSubjectOpen} onOpenChange={setIsSubjectOpen}>
        <DialogContent>
          <form onSubmit={handleAddSubject}>
            <DialogHeader>
              <DialogTitle>Add Subject</DialogTitle>
              <DialogDescription>Define who this policy applies to</DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="subject_type">Subject Type</Label>
                <select
                  id="subject_type"
                  className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                  value={newSubject.subject_type}
                  onChange={(e) => setNewSubject({
                    ...newSubject,
                    subject_type: e.target.value as any,
                    subject_value: e.target.value === "everyone" ? "" : newSubject.subject_value,
                  })}
                >
                  <option value="everyone">Everyone</option>
                  <option value="role">Role</option>
                  <option value="group">Group</option>
                  <option value="user">User (ID)</option>
                </select>
              </div>
              {newSubject.subject_type !== "everyone" && (
                <div className="space-y-2">
                  <Label htmlFor="subject_value">
                    {newSubject.subject_type === "user" ? "User ID" : newSubject.subject_type === "role" ? "Role Name" : "Group Name"}
                  </Label>
                  <Input
                    id="subject_value"
                    placeholder={
                      newSubject.subject_type === "user" ? "Enter user UUID" :
                        newSubject.subject_type === "role" ? "e.g., admin" : "e.g., developers"
                    }
                    value={newSubject.subject_value}
                    onChange={(e) => setNewSubject({ ...newSubject, subject_value: e.target.value })}
                    required
                  />
                </div>
              )}
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setIsSubjectOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={addSubjectMutation.isPending}>
                {addSubjectMutation.isPending && <Loader2 className="h-4 w-4 mr-2 animate-spin" />}
                Add
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </DashboardLayout>
  );
}
