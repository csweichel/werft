import * as React from 'react';
import { withStyles, createStyles, WithStyles } from '@material-ui/styles';
import { Theme, fade, Chip, Tooltip } from '@material-ui/core';
import SearchIcon from '@material-ui/icons/Search';
import ChipInput from 'material-ui-chip-input';
import { FilterExpression, FilterTerm, FilterOp, FilterOpMap } from '../api/werft_pb';

export const styles = (theme: Theme) =>
    createStyles({
        search: {
            position: 'relative',
            borderRadius: theme.shape.borderRadius,
            backgroundColor: fade(theme.palette.common.white, 0.15),
            '&:hover': {
                backgroundColor: fade(theme.palette.common.white, 0.25),
            },
            marginLeft: 0,
            width: '100%',
        },
        searchIcon: {
            width: theme.spacing(7),
            height: '100%',
            position: 'absolute',
            pointerEvents: 'none',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
        },
        inputRoot: {
            color: 'inherit',
            width: '100%',
            padding: theme.spacing(0, 0, 0, 7),
        },
        inputInput: {
            width: '100%',
            color: 'white'
        },
        chip: {
            margin: "3px 8px 0px 0px"
        }
    });

export interface SearchBoxProps extends WithStyles<typeof styles> {
    onUpdate: (expr: FilterExpression[]) => void
    defaultValue?: string[]
}

interface SearchBoxState {
    errors: (string | undefined)[]
}

class SearchBoxImpl extends React.Component<SearchBoxProps, SearchBoxState> {

    constructor(p: SearchBoxProps) {
        super(p);
        this.state = { errors: [] };
    }

    componentDidMount() {
        console.log(this.props.defaultValue);

        if (!!this.props.defaultValue) {
            this.updateChips(this.props.defaultValue);
        }
    }

    render() {
        const classes = this.props.classes;
        return <div className={classes.search}>
            <div className={classes.searchIcon}>
                <SearchIcon />
            </div>
            <ChipInput classes={{ input: classes.inputInput }} 
                className={classes.inputRoot}
                disableUnderline 
                placeholder="Search"
                dataSource={["branch:", "name:"]}
                defaultValue={this.props.defaultValue}
                onChange={chips => this.updateChips(chips)}
                chipRenderer={(args, key) => <Tooltip title={this.state.errors[key] || ""} key={key}>
                    <Chip 
                        label={args.text} 
                        onClick={args.handleClick} 
                        onDelete={args.handleDelete} 
                        className={classes.chip} 
                        color={!!this.state.errors[key] ? "secondary" : "default"}
                    />
                </Tooltip>}
                 />
        </div>
    }

    protected updateChips(newChips: string[]) {
        const validFields = ['name', 'owner', 'repo.repo', 'repo.ref', 'phase', 'success'];
        const operations: { [op: string]: FilterOpMap[keyof FilterOpMap] } = {
            "==": FilterOp.OP_EQUALS,
            "~=": FilterOp.OP_CONTAINS,
            "|=": FilterOp.OP_STARTS_WITH,
            "=|": FilterOp.OP_ENDS_WITH,
        }

        const parse = (chp: string): { error?: string, expr?: FilterExpression } => {
            if (chp === "success" || chp === "!success") {
                const expr = new FilterExpression();
                const tf = new FilterTerm();
                tf.setField("success");
                tf.setOperation(FilterOp.OP_EQUALS);
                tf.setValue(chp.startsWith("!") ? "0" : "1");
                expr.setTermsList([ tf ]);
                return { expr: expr };
            }

            const includedOp = Object.getOwnPropertyNames(operations).find(op => chp.includes(op));
            if (!!includedOp) {
                const [l, r] = chp.split(includedOp);
                let [ field, op, val ] = [ l.trim(), includedOp, r.trim() ];

                if (field === "ref" || field === "branch") {
                    field = "repo.ref";
                }

                if (!validFields.includes(field)) {
                    return { error: "unknown field" }
                }
                if (operations[op] === undefined) {
                    return { error: "unknown operator" }
                }

                const expr = new FilterExpression();
                const tf = new FilterTerm();
                tf.setField(field);
                tf.setOperation(operations[op]);
                tf.setValue(val);
                expr.setTermsList([ tf ]);
                return { expr: expr };
            }

            const terms = validFields.filter(f => f !== "success").map(f => {
                const tt = new FilterTerm();
                tt.setField(f);
                tt.setOperation(FilterOp.OP_CONTAINS);
                tt.setValue(chp);
                return tt;
            });
            const tf = new FilterExpression();
            tf.setTermsList(terms);
            return { expr: tf };
        }
        const pr = newChips.map(parse);

        this.setState({ errors: pr.map(e => e.error) });

        const expressions = pr.map(e => e.expr).filter(e => !!e).map(e => e!);
        this.props.onUpdate(expressions);
    }

}

export const SearchBox = withStyles(styles)(SearchBoxImpl);