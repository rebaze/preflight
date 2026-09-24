ARG BASE
FROM ${BASE}
RUN npm install --global npm@11.17.0 --ignore-scripts --no-audit --fund=false --registry=https://registry.npmjs.org && test "$(node --version)" = v24.18.0 && test "$(npm --version)" = 11.17.0
COPY run-tests.sh /usr/local/bin/preflight-run-tests
RUN chmod 0555 /usr/local/bin/preflight-run-tests
WORKDIR /work/applications/frontend
ENTRYPOINT ["/usr/local/bin/preflight-run-tests"]
