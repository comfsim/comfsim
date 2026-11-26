import { CharacterTile } from "@gcsim/components";
import LatestCharactersData from "@gcsim/data/src/latest_chars.json";
import { cn } from "@gcsim/primitives";
import axios from "axios";
import { ArrowRight } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

const majorVersionRegex = /v\d+\.\d+/;

type GithubCommit = {
	sha: string;
	html_url: string;
	commit: {
		message: string;
	};
};

export function WhatsNew({ className }: { className?: string }) {
	const { t } = useTranslation();

	const [commits, setCommits] = useState<GithubCommit[]>([]);
	const [isLoaded, setIsLoaded] = useState(false);

	useEffect(() => {
		axios
			.get<GithubCommit[]>(
				"https://api.github.com/repos/comfsim/comfsim/commits?per_page=5",
			)
			.then(({ data }) => setCommits(data))
			.catch((err) => console.error(err.message))
			.finally(() => setIsLoaded(true));
	}, []);

	const cardClass = cn(
		"border border-g-line bg-g-surface rounded-g-lg p-g-card shadow-g-card",
		className,
	);

	if (!isLoaded) {
		return <div className={cardClass}>{t("sim.loading")}</div>;
	}

	const currentCommit = commits[0];

	if (!currentCommit) {
		return <div className={cardClass}>{"Failed to load chanegs :("}</div>;
	}

	return (
		<div className={cardClass}>
			<p className="mb-3 text-g-sm text-g-ink-mute">
				Current commit:{" "}
				<a
					href={currentCommit.html_url}
					target="_blank"
					rel="noreferrer"
					className="font-g-mono text-g-accent hover:text-g-accent-hover"
				>
					{currentCommit.sha.slice(0, 7)}
				</a>
			</p>

			<ul className="list-disc space-y-1 pl-5 text-g-body text-g-ink-dim">
				{commits.map((commit) => (
					<li key={commit.sha}>
						<a
							href={commit.html_url}
							target="_blank"
							rel="noreferrer"
							className="hover:text-g-accent"
						>
							{commit.commit.message.split("\n")[0]}
						</a>
					</li>
				))}
			</ul>
		</div>
	);
}
