import { Component, type ReactNode } from "react";

interface Props {
  children: ReactNode;
}

interface State {
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <div className="sched">
          <div className="note" role="alert">
            <span>
              <strong>Can’t reach the Bitswarm server.</strong> {this.state.error.message}. Start it with{" "}
              <code>./run.sh</code>, then{" "}
              <button className="btn" onClick={() => this.setState({ error: null })}>
                Try again
              </button>
            </span>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}
