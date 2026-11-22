// Keeps the patches branch on the latest upstream release and tags a release
// for each new state of it. The goreleaser step builds the tag this outputs.
type Core = {
	info: (message: string) => void;
	setFailed: (message: string) => void;
	setOutput: (name: string, value: string) => void;
};
type ExecOptions = { cwd?: string; ignoreReturnCode?: boolean };
type Exec = {
	exec: (
		command: string,
		args: string[],
		options?: ExecOptions,
	) => Promise<number>;
	getExecOutput: (
		command: string,
		args: string[],
		options?: ExecOptions,
	) => Promise<{ stdout: string }>;
};
type GitHub = {
	rest: {
		repos: {
			getLatestRelease: (params: {
				owner: string;
				repo: string;
			}) => Promise<{ data: { tag_name: string } }>;
		};
	};
};

const UPSTREAM = { owner: "kreuzwerker", repo: "terraform-provider-docker" };
const BRANCH = "patches";
// The branch is checked out here by the workflow
const cwd = BRANCH;
// v4.7.0 + this = v4.7.0-fw.1, v4.7.0-fw.2, ...
const SUFFIX = "-fw.";

export const maintain = async ({
	github,
	core,
	exec,
}: {
	github: GitHub;
	core: Core;
	exec: Exec;
}) => {
	const git = async (...args: string[]) =>
		(await exec.getExecOutput("git", args, { cwd })).stdout.trim();
	const succeeds = async (command: string, ...args: string[]) =>
		(await exec.exec(command, args, { cwd, ignoreReturnCode: true })) === 0;

	const {
		data: { tag_name: upstream },
	} = await github.rest.repos.getLatestRelease(UPSTREAM);
	core.setOutput("upstream", upstream);
	await git(
		"fetch",
		"--no-tags",
		`https://github.com/${UPSTREAM.owner}/${UPSTREAM.repo}.git`,
		`refs/tags/${upstream}:refs/tags/${upstream}`,
	);

	const before = await git("rev-parse", "HEAD");
	const onUpstream = await succeeds(
		"git",
		"merge-base",
		"--is-ancestor",
		upstream,
		"HEAD",
	);
	if (!onUpstream && !(await succeeds("git", "rebase", upstream))) {
		await git("rebase", "--abort");
		core.setFailed(
			`The commits on ${BRANCH} do not apply onto ${upstream}. Rebase ${BRANCH} onto it by hand.`,
		);
		return;
	}

	const prefix = `${upstream}${SUFFIX}`;
	const released = await git(
		"tag",
		"--list",
		`${prefix}*`,
		"--points-at",
		"HEAD",
	);
	if (released) {
		core.info(`${released} is already ${BRANCH} as it is.`);
		return;
	}

	// The tests of the patches themselves; the rest need a Docker daemon
	if (
		!(await succeeds("go", "test", "./internal/provider", "-run", "DockerHost"))
	) {
		core.setFailed(`The tests of the patches fail on ${upstream}.`);
		return;
	}

	if (!onUpstream) {
		await git(
			"push",
			`--force-with-lease=${BRANCH}:${before}`,
			"origin",
			`HEAD:refs/heads/${BRANCH}`,
		);
	}

	const numbers = (await git("tag", "--list", `${prefix}*`))
		.split("\n")
		.map((tag) => Number(tag.slice(prefix.length)))
		.filter(Number.isInteger);
	const tag = `${prefix}${Math.max(0, ...numbers) + 1}`;
	await git("tag", tag);
	// Pushed with the workflow's own token, so no workflow on the tag runs,
	// including upstream's release workflow that wants a GPG key.
	await git("push", "origin", `refs/tags/${tag}`);
	core.setOutput("tag", tag);
};
